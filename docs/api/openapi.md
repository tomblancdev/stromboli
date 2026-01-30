# OpenAPI Specification

Interactive API documentation and downloadable specifications.

## Swagger UI (Interactive)

Try out the API directly from your browser:

<div id="swagger-ui"></div>

<script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-bundle.js"></script>
<link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5/swagger-ui.css" />
<script>
  window.onload = function() {
    SwaggerUIBundle({
      url: "../swagger/swagger.yaml",
      dom_id: '#swagger-ui',
      presets: [
        SwaggerUIBundle.presets.apis,
        SwaggerUIBundle.SwaggerUIStandalonePreset
      ],
      layout: "BaseLayout",
      deepLinking: true,
      showExtensions: true,
      showCommonExtensions: true,
      defaultModelsExpandDepth: 1,
      defaultModelExpandDepth: 1,
      docExpansion: "list",
      filter: true,
      tryItOutEnabled: false
    });
  };
</script>

<style>
  #swagger-ui {
    border: 1px solid var(--md-default-fg-color--lightest);
    border-radius: 4px;
    margin: 1em 0;
  }
  .swagger-ui .topbar { display: none; }
  .swagger-ui .info { margin: 20px 0; }
  .swagger-ui .scheme-container { display: none; }
</style>

---

## ReDoc (Documentation View)

Clean, readable API documentation:

<div id="redoc-container"></div>

<script src="https://cdn.redoc.ly/redoc/latest/bundles/redoc.standalone.js"></script>
<script>
  Redoc.init('../swagger/swagger.yaml', {
    scrollYOffset: 60,
    hideDownloadButton: false,
    expandResponses: "200,201",
    pathInMiddlePanel: true,
    theme: {
      colors: {
        primary: { main: '#673ab7' }
      },
      typography: {
        fontSize: '15px',
        fontFamily: 'inherit'
      },
      sidebar: {
        backgroundColor: 'transparent'
      }
    }
  }, document.getElementById('redoc-container'));
</script>

<style>
  #redoc-container {
    margin: 1em 0;
  }
</style>

---

## Download Specifications

Download the raw OpenAPI specification files:

| Format | Download | Description |
|--------|----------|-------------|
| **YAML** | [swagger.yaml](../swagger/swagger.yaml) | Human-readable format |
| **JSON** | [swagger.json](../swagger/swagger.json) | Machine-readable format |

### Usage Examples

**Import into Postman:**
```bash
# Download and import
curl -O https://tomblancdev.github.io/stromboli/swagger/swagger.yaml
# Then import in Postman: File > Import > Upload Files
```

**Generate client SDK:**
```bash
# Using openapi-generator
openapi-generator generate \
  -i https://tomblancdev.github.io/stromboli/swagger/swagger.yaml \
  -g python \
  -o ./stromboli-client

# Using swagger-codegen
swagger-codegen generate \
  -i https://tomblancdev.github.io/stromboli/swagger/swagger.yaml \
  -l javascript \
  -o ./stromboli-js-client
```

**Validate the spec:**
```bash
# Using swagger-cli
npx @apidevtools/swagger-cli validate swagger.yaml

# Using spectral
npx @stoplight/spectral-cli lint swagger.yaml
```

---

## API Versioning

The current API version is **v1.0**.

| Version | Status | Spec |
|---------|--------|------|
| v1.0 | Current | [swagger.yaml](../swagger/swagger.yaml) |

!!! note "Breaking Changes"
    Breaking changes will be announced in the [Changelog](../changelog.md) and will increment the major version number.

## Live Server

When running Stromboli locally, the spec is also available at:

```
http://localhost:8080/swagger/doc.json
```
