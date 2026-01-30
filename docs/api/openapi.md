# OpenAPI Specification

API documentation and downloadable specifications.

## Interactive Documentation

View the API documentation using ReDoc:

<div class="grid cards" markdown>

-   :material-file-document: **ReDoc Viewer**

    ---

    Clean, readable API documentation

    [:octicons-link-external-16: Open ReDoc](https://redocly.github.io/redoc/?url=https://tomblancdev.github.io/stromboli/latest/swagger/swagger.yaml){ .md-button .md-button--primary target="_blank" }

</div>

!!! tip "Versioned Documentation"
    The ReDoc link above uses the **latest** version. For a specific version, use:

    ```
    https://redocly.github.io/redoc/?url=https://tomblancdev.github.io/stromboli/VERSION/swagger/swagger.yaml
    ```

    Replace `VERSION` with `0.2.0`, `latest`, etc.

## Download Specifications

Download the raw OpenAPI specification files:

| Format | Latest | v0.2.0 |
|--------|--------|--------|
| **YAML** | [swagger.yaml](https://tomblancdev.github.io/stromboli/latest/swagger/swagger.yaml){ target="_blank" } | [swagger.yaml](https://tomblancdev.github.io/stromboli/0.2.0/swagger/swagger.yaml){ target="_blank" } |
| **JSON** | [swagger.json](https://tomblancdev.github.io/stromboli/latest/swagger/swagger.json){ target="_blank" } | [swagger.json](https://tomblancdev.github.io/stromboli/0.2.0/swagger/swagger.json){ target="_blank" } |

## Usage Examples

### Import into Postman

```bash
# Download the spec
curl -O https://tomblancdev.github.io/stromboli/latest/swagger/swagger.yaml

# Then in Postman: File > Import > Upload Files
```

### Generate Client SDK

```bash
# Using openapi-generator (Python client)
openapi-generator generate \
  -i https://tomblancdev.github.io/stromboli/latest/swagger/swagger.yaml \
  -g python \
  -o ./stromboli-client

# Using openapi-generator (TypeScript client)
openapi-generator generate \
  -i https://tomblancdev.github.io/stromboli/latest/swagger/swagger.yaml \
  -g typescript-fetch \
  -o ./stromboli-ts-client
```

### Validate the Spec

```bash
# Using spectral
npx @stoplight/spectral-cli lint \
  https://tomblancdev.github.io/stromboli/latest/swagger/swagger.yaml
```

## Versioned URLs

Each Stromboli release has its own OpenAPI spec:

| Version | ReDoc | YAML | JSON |
|---------|-------|------|------|
| **latest** | [View](https://redocly.github.io/redoc/?url=https://tomblancdev.github.io/stromboli/latest/swagger/swagger.yaml){ target="_blank" } | [Download](https://tomblancdev.github.io/stromboli/latest/swagger/swagger.yaml){ target="_blank" } | [Download](https://tomblancdev.github.io/stromboli/latest/swagger/swagger.json){ target="_blank" } |
| **0.2.0** | [View](https://redocly.github.io/redoc/?url=https://tomblancdev.github.io/stromboli/0.2.0/swagger/swagger.yaml){ target="_blank" } | [Download](https://tomblancdev.github.io/stromboli/0.2.0/swagger/swagger.yaml){ target="_blank" } | [Download](https://tomblancdev.github.io/stromboli/0.2.0/swagger/swagger.json){ target="_blank" } |

## Local Server

When running Stromboli locally, the spec is also available at:

```
http://localhost:8080/swagger/doc.json
```
