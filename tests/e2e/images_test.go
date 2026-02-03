//go:build e2e

package e2e

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestImages_List tests listing local images
func TestImages_List(t *testing.T) {
	env := setupE2EEnv(t)

	resp := makeRequest(t, "GET", env.BaseURL+"/images", nil, nil)
	assertStatusCode(t, resp, http.StatusOK)

	var listResp map[string]interface{}
	readJSONResponse(t, resp, &listResp)

	// Verify images field exists and is an array
	images, ok := listResp["images"]
	require.True(t, ok, "response should have images field")
	require.NotNil(t, images, "images should not be null")

	imageList, ok := images.([]interface{})
	require.True(t, ok, "images should be an array")

	// If there are images, verify their structure
	if len(imageList) > 0 {
		img := imageList[0].(map[string]interface{})
		assert.NotEmpty(t, img["id"], "image should have id")
		assert.NotEmpty(t, img["repository"], "image should have repository")
		assert.Contains(t, img, "tag", "image should have tag")
		assert.Contains(t, img, "size", "image should have size")
		assert.Contains(t, img, "compatibility_rank", "image should have compatibility_rank")
		assert.Contains(t, img, "compatible", "image should have compatible")
	}
}

// TestImages_ListSorted tests that images are sorted by compatibility
func TestImages_ListSorted(t *testing.T) {
	env := setupE2EEnv(t)

	resp := makeRequest(t, "GET", env.BaseURL+"/images", nil, nil)
	assertStatusCode(t, resp, http.StatusOK)

	var listResp map[string]interface{}
	readJSONResponse(t, resp, &listResp)

	imageList := listResp["images"].([]interface{})
	if len(imageList) < 2 {
		t.Skip("need at least 2 images to test sorting")
	}

	// Verify images are sorted by compatibility rank (lower is better)
	prevRank := 0
	for _, img := range imageList {
		imgMap := img.(map[string]interface{})
		rank := int(imgMap["compatibility_rank"].(float64))
		assert.GreaterOrEqual(t, rank, prevRank, "images should be sorted by compatibility rank (ascending)")
		prevRank = rank
	}
}

// TestImages_Inspect tests inspecting a specific image
func TestImages_Inspect(t *testing.T) {
	env := setupE2EEnv(t)

	// First list images to get a valid image name
	resp := makeRequest(t, "GET", env.BaseURL+"/images", nil, nil)
	assertStatusCode(t, resp, http.StatusOK)

	var listResp map[string]interface{}
	readJSONResponse(t, resp, &listResp)

	imageList := listResp["images"].([]interface{})
	if len(imageList) == 0 {
		t.Skip("no local images available for inspection")
	}

	// Get first image name
	firstImg := imageList[0].(map[string]interface{})
	imageName := firstImg["repository"].(string)
	if tag, ok := firstImg["tag"].(string); ok && tag != "" {
		imageName = imageName + ":" + tag
	}

	// Inspect the image
	resp = makeRequest(t, "GET", env.BaseURL+"/images/"+imageName, nil, nil)
	assertStatusCode(t, resp, http.StatusOK)

	var inspectResp map[string]interface{}
	readJSONResponse(t, resp, &inspectResp)

	// Verify detailed response
	assert.NotEmpty(t, inspectResp["id"])
	assert.NotEmpty(t, inspectResp["repository"])
	assert.Contains(t, inspectResp, "labels", "inspect should include labels")
	assert.Contains(t, inspectResp, "rank_description", "inspect should include rank_description")
}

// TestImages_InspectNotFound tests inspecting a non-existent image
func TestImages_InspectNotFound(t *testing.T) {
	env := setupE2EEnv(t)

	resp := makeRequest(t, "GET", env.BaseURL+"/images/nonexistent-image-12345", nil, nil)
	assertStatusCode(t, resp, http.StatusNotFound)

	var errorResp map[string]interface{}
	readJSONResponse(t, resp, &errorResp)
	assert.Contains(t, errorResp["error"], "not found")
}

// TestImages_Search tests searching for images in registries
func TestImages_Search(t *testing.T) {
	env := setupE2EEnv(t)

	resp := makeRequest(t, "GET", env.BaseURL+"/images/search?q=alpine", nil, nil)
	assertStatusCode(t, resp, http.StatusOK)

	var searchResp map[string]interface{}
	readJSONResponse(t, resp, &searchResp)

	// Verify results field exists
	results, ok := searchResp["results"]
	require.True(t, ok, "response should have results field")
	require.NotNil(t, results, "results should not be null")

	resultList := results.([]interface{})
	if len(resultList) > 0 {
		result := resultList[0].(map[string]interface{})
		assert.Contains(t, result, "name", "result should have name")
		assert.Contains(t, result, "description", "result should have description")
		assert.Contains(t, result, "stars", "result should have stars")
	}
}

// TestImages_SearchWithLimit tests searching with limit parameter
func TestImages_SearchWithLimit(t *testing.T) {
	env := setupE2EEnv(t)

	resp := makeRequest(t, "GET", env.BaseURL+"/images/search?q=python&limit=5", nil, nil)
	assertStatusCode(t, resp, http.StatusOK)

	var searchResp map[string]interface{}
	readJSONResponse(t, resp, &searchResp)

	results := searchResp["results"].([]interface{})
	assert.LessOrEqual(t, len(results), 5, "results should respect limit parameter")
}

// TestImages_SearchMissingQuery tests searching without query parameter
func TestImages_SearchMissingQuery(t *testing.T) {
	env := setupE2EEnv(t)

	resp := makeRequest(t, "GET", env.BaseURL+"/images/search", nil, nil)
	assertStatusCode(t, resp, http.StatusBadRequest)

	var errorResp map[string]interface{}
	readJSONResponse(t, resp, &errorResp)
	assert.Contains(t, errorResp["error"], "query parameter")
}

// TestImages_SearchInvalidLimit tests searching with invalid limit
func TestImages_SearchInvalidLimit(t *testing.T) {
	env := setupE2EEnv(t)

	tests := []struct {
		name  string
		limit string
	}{
		{"non-numeric", "abc"},
		{"zero", "0"},
		{"negative", "-5"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := makeRequest(t, "GET", env.BaseURL+"/images/search?q=test&limit="+tt.limit, nil, nil)
			assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
			resp.Body.Close()
		})
	}
}

// TestImages_Pull tests pulling an image from registry
func TestImages_Pull(t *testing.T) {
	env := setupE2EEnv(t)

	// Pull a small image for the test
	pullReq := map[string]interface{}{
		"image": "busybox:latest",
		"quiet": true,
	}

	resp := makeRequest(t, "POST", env.BaseURL+"/images/pull", pullReq, nil)

	// Pull may take time, but should eventually succeed or fail gracefully
	if resp.StatusCode == http.StatusOK {
		var pullResp map[string]interface{}
		readJSONResponse(t, resp, &pullResp)
		assert.True(t, pullResp["success"].(bool))
		assert.NotEmpty(t, pullResp["image_id"])
	} else {
		// May fail due to network issues in CI, which is acceptable
		assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
		resp.Body.Close()
	}
}

// TestImages_PullMissingImage tests pulling without image name
func TestImages_PullMissingImage(t *testing.T) {
	env := setupE2EEnv(t)

	pullReq := map[string]interface{}{}

	resp := makeRequest(t, "POST", env.BaseURL+"/images/pull", pullReq, nil)
	assertStatusCode(t, resp, http.StatusBadRequest)
}

// TestImages_PullInvalidRequest tests pulling with invalid JSON
func TestImages_PullInvalidRequest(t *testing.T) {
	env := setupE2EEnv(t)

	// Send invalid JSON
	req, _ := http.NewRequest("POST", env.BaseURL+"/images/pull", nil)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}
