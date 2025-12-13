# Article Content Type - Field Configuration Guide

## Current Fields
- **Group** (`field_group`) - Entity reference to Group (News type) - Required
- **URL** (`field_url`) - Link field - Required

## Recommended Additional Fields

### 1. Body (Standard Field - May Already Exist)
- **Field Type**: Text (formatted, long)
- **Machine Name**: `body` (standard Drupal field)
- **Widget**: Text area (multiple rows)
- **Required**: Optional
- **Settings**:
  - Allowed text formats: Basic HTML, Full HTML
  - Default text format: Basic HTML
- **Purpose**: Main article content/body text
- **API Usage**:
  ```json
  "body": {
    "value": "<p>Article content here</p>",
    "format": "basic_html"
  }
  ```

### 2. Summary/Excerpt
- **Field Type**: Text (formatted, long)
- **Machine Name**: `field_summary`
- **Widget**: Text area (multiple rows)
- **Required**: Optional
- **Settings**:
  - Allowed text formats: Plain text or Basic HTML
  - Default text format: Plain text
- **Purpose**: Short summary/excerpt for teasers and listings
- **API Usage**:
  ```json
  "field_summary": {
    "value": "Brief summary of the article",
    "format": "plain_text"
  }
  ```

### 3. Featured Image
- **Field Type**: Image
- **Machine Name**: `field_featured_image`
- **Widget**: Image
- **Required**: Optional
- **Settings**:
  - Allowed file extensions: `jpg jpeg png gif webp`
  - Maximum upload size: 2MB (adjust as needed)
  - Image style: Large (for display)
- **Purpose**: Main image for the article
- **API Usage**: Upload via file endpoint first, then reference:
  ```json
  "field_featured_image": {
    "data": {
      "type": "file--file",
      "id": "uuid-of-uploaded-file"
    }
  }
  ```

### 4. Tags/Categories
- **Field Type**: Entity reference (Taxonomy term)
- **Machine Name**: `field_tags`
- **Widget**: Check boxes/Select list
- **Required**: Optional
- **Settings**:
  - Reference type: Taxonomy term
  - Target bundles: Tags (or create a new vocabulary)
  - Unlimited values: Yes (if using checkboxes)
- **Purpose**: Categorization and tagging
- **API Usage**:
  ```json
  "field_tags": {
    "data": [
      {
        "type": "taxonomy_term--tags",
        "id": "term-uuid-1"
      },
      {
        "type": "taxonomy_term--tags",
        "id": "term-uuid-2"
      }
    ]
  }
  ```

### 5. Author/Byline (if different from node author)
- **Field Type**: Text (plain)
- **Machine Name**: `field_author_name`
- **Widget**: Text field
- **Required**: Optional
- **Settings**:
  - Maximum length: 255
- **Purpose**: Display author name (if different from Drupal user)
- **API Usage**:
  ```json
  "field_author_name": {
    "value": "John Doe"
  }
  ```

### 6. Publication Date (if different from created date)
- **Field Type**: Date
- **Machine Name**: `field_publication_date`
- **Widget**: Date picker
- **Required**: Optional
- **Settings**:
  - Date type: Date only or Date and time
  - Default value: None
- **Purpose**: Custom publication date
- **API Usage**:
  ```json
  "field_publication_date": {
    "value": "2024-01-15T10:00:00"
  }
  ```

### 7. External Source URL (if different from field_url)
- **Field Type**: Link
- **Machine Name**: `field_source_url`
- **Widget**: Link
- **Required**: Optional
- **Settings**:
  - Link type: External
  - Title: Optional
- **Purpose**: Link to original source
- **API Usage**:
  ```json
  "field_source_url": {
    "uri": "https://example.com/article",
    "title": "Read more"
  }
  ```

## Step-by-Step Instructions

### Adding Fields via UI:

1. **Navigate to**: `/admin/structure/types/manage/article/fields`

2. **For each field**:
   - Click "Add field" or "Re-use an existing field"
   - Select field type
   - Enter label and machine name
   - Configure settings
   - Save

3. **Configure Form Display**: `/admin/structure/types/manage/article/form-display`
   - Arrange field order
   - Configure widget settings
   - Set required fields

4. **Configure Display**: `/admin/structure/types/manage/article/display`
   - Configure how fields appear in view mode
   - Set label display
   - Configure formatters

### Adding Fields via Drush:

```bash
# Create Summary field
ddev drush field:create node article field_summary \
  --field-type=text_long \
  --label="Summary" \
  --required=false

# Create Featured Image field
ddev drush field:create node article field_featured_image \
  --field-type=image \
  --label="Featured Image" \
  --required=false

# Create Tags field (requires taxonomy vocabulary first)
ddev drush field:create node article field_tags \
  --field-type=entity_reference \
  --label="Tags" \
  --required=false
```

## JSON:API Field Configuration

After adding fields, ensure they're exposed in JSON:API:

1. Go to: `/admin/config/services/jsonapi/extras/node--article`
2. Check that new fields appear in the resource fields list
3. Ensure they're enabled (not disabled)
4. Configure field names if needed

## Complete API Example with All Fields

```json
{
  "data": {
    "type": "node--article",
    "attributes": {
      "title": "My Article Title",
      "body": {
        "value": "<p>Full article content here...</p>",
        "format": "basic_html"
      },
      "field_summary": {
        "value": "Brief summary",
        "format": "plain_text"
      },
      "field_author_name": "John Doe",
      "field_publication_date": "2024-01-15T10:00:00",
      "field_url": {
        "uri": "https://example.com",
        "title": "Example"
      },
      "field_source_url": {
        "uri": "https://source.com/article",
        "title": "Source"
      },
      "status": true,
      "promote": false,
      "sticky": false
    },
    "relationships": {
      "field_group": {
        "data": {
          "type": "group--news",
          "id": "e3d024a6-5f6f-4be8-8f3d-75639075959c"
        }
      },
      "field_featured_image": {
        "data": {
          "type": "file--file",
          "id": "file-uuid-here"
        }
      },
      "field_tags": {
        "data": [
          {
            "type": "taxonomy_term--tags",
            "id": "term-uuid-1"
          }
        ]
      }
    }
  }
}
```

## Field Priority Recommendations

**Essential for API:**
1. ✅ Group (already exists)
2. ✅ URL (already exists)
3. Body (standard field - check if exists)
4. Summary/Excerpt (highly recommended)

**Recommended:**
5. Featured Image
6. Tags/Categories

**Optional:**
7. Author Name
8. Publication Date
9. Source URL

## Notes

- **Body field**: Check if it already exists at `/admin/structure/types/manage/article/fields` - it's a standard Drupal field
- **Required fields**: Only make fields required if absolutely necessary for your use case
- **Field names**: Use descriptive machine names following Drupal conventions (`field_*`)
- **API compatibility**: All field types listed above are fully compatible with JSON:API

