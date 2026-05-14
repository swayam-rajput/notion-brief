package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

type NotionPage struct {
	ID             string
	Title          string
	LastEditedTime string
	URL            string
}

type RichText struct {
	PlainText string `json:"plain_text"`
}

type NotionBlock struct {
	ID          string `json:"id"`
	Type        string `json:"type"`
	HasChildren bool   `json:"has_children"`

	Paragraph *struct {
		RichText []RichText `json:"rich_text"`
	} `json:"paragraph"`

	Heading1 *struct {
		RichText []RichText `json:"rich_text"`
	} `json:"heading_1"`

	Heading2 *struct {
		RichText []RichText `json:"rich_text"`
	} `json:"heading_2"`

	Heading3 *struct {
		RichText []RichText `json:"rich_text"`
	} `json:"heading_3"`

	BulletedListItem *struct {
		RichText []RichText `json:"rich_text"`
	} `json:"bulleted_list_item"`

	NumberedListItem *struct {
		RichText []RichText `json:"rich_text"`
	} `json:"numbered_list_item"`

	ToDo *struct {
		RichText []RichText `json:"rich_text"`
		Checked  bool       `json:"checked"`
	} `json:"to_do"`

	Quote *struct {
		RichText []RichText `json:"rich_text"`
	} `json:"quote"`

	Callout *struct {
		RichText []RichText `json:"rich_text"`
	} `json:"callout"`
}

func fetchNotionPages() ([]NotionPage, error) {
	key := os.Getenv("NOTION_API_KEY")
	if key == "" {
		return nil, fmt.Errorf("NOTION_API_KEY not set")
	}

	dbID := os.Getenv("NOTION_DATABASE_ID")

	var (
		url     string
		payload string
		method  string
	)

	if dbID != "" {
		url = fmt.Sprintf("https://api.notion.com/v1/databases/%s/query", dbID)
		payload = `{
			"page_size": 20,
			"sorts": [
				{
					"timestamp": "last_edited_time",
					"direction": "descending"
				}
			]
		}`
		method = "POST"
	} else {
		url = "https://api.notion.com/v1/search"
		payload = `{
			"page_size": 20,
			"filter": {
				"value": "page",
				"property": "object"
			}
		}`
		method = "POST"
	}

	body, err := notionRequest(method, url, strings.NewReader(payload))
	if err != nil {
		return nil, err
	}

	var data struct {
		Results []struct {
			ID             string `json:"id"`
			LastEditedTime string `json:"last_edited_time"`
			URL            string `json:"url"`
			Properties     map[string]struct {
				Title []struct {
					PlainText string `json:"plain_text"`
				} `json:"title"`
				RichText []struct {
					PlainText string `json:"plain_text"`
				} `json:"rich_text"`
			} `json:"properties"`
		} `json:"results"`
	}

	if err := json.Unmarshal(body, &data); err != nil {
		return nil, err
	}

	var pages []NotionPage
	for _, r := range data.Results {
		title := "Untitled"
		for _, propName := range []string{"Name", "Title", "title", "name"} {
			prop, ok := r.Properties[propName]
			if !ok {
				continue
			}
			for _, t := range prop.Title {
				if t.PlainText != "" {
					title = t.PlainText
					break
				}
			}
			if title != "Untitled" {
				break
			}
		}

		pages = append(pages, NotionPage{
			ID:             r.ID,
			Title:          title,
			LastEditedTime: r.LastEditedTime,
			URL:            r.URL,
		})
	}

	return pages, nil
}

func fetchPageContent(pageID string) (string, error) {
	return fetchBlocksRecursive(pageID, 0)
}

func fetchBlocksRecursive(blockID string, depth int) (string, error) {
	url := fmt.Sprintf("https://api.notion.com/v1/blocks/%s/children?page_size=100", blockID)
	body, err := notionRequest("GET", url, nil)
	if err != nil {
		return "", err
	}

	var data struct {
		Results []NotionBlock `json:"results"`
	}
	if err := json.Unmarshal(body, &data); err != nil {
		return "", err
	}

	var content strings.Builder
	indent := strings.Repeat("  ", depth)

	for _, block := range data.Results {
		var text string
		switch block.Type {
		case "paragraph":
			if block.Paragraph != nil {
				text = flattenRichText(block.Paragraph.RichText)
			}
		case "heading_1":
			if block.Heading1 != nil {
				text = "# " + flattenRichText(block.Heading1.RichText)
			}
		case "heading_2":
			if block.Heading2 != nil {
				text = "## " + flattenRichText(block.Heading2.RichText)
			}
		case "heading_3":
			if block.Heading3 != nil {
				text = "### " + flattenRichText(block.Heading3.RichText)
			}
		case "bulleted_list_item":
			if block.BulletedListItem != nil {
				text = "• " + flattenRichText(block.BulletedListItem.RichText)
			}
		case "numbered_list_item":
			if block.NumberedListItem != nil {
				text = "1. " + flattenRichText(block.NumberedListItem.RichText)
			}
		case "to_do":
			if block.ToDo != nil {
				status := "[ ] "
				if block.ToDo.Checked {
					status = "[x] "
				}
				text = status + flattenRichText(block.ToDo.RichText)
			}
		case "quote":
			if block.Quote != nil {
				text = "> " + flattenRichText(block.Quote.RichText)
			}
		case "callout":
			if block.Callout != nil {
				text = "💡 " + flattenRichText(block.Callout.RichText)
			}
		}

		if text != "" {
			content.WriteString(indent + text + "\n")
		}

		if block.HasChildren {
			childContent, err := fetchBlocksRecursive(block.ID, depth+1)
			if err == nil {
				content.WriteString(childContent)
			}
		}
	}

	return content.String(), nil
}

func flattenRichText(richText []RichText) string {
	var sb strings.Builder
	for _, t := range richText {
		sb.WriteString(t.PlainText)
	}
	return sb.String()
}

func notionRequest(method, url string, body io.Reader) ([]byte, error) {
	key := os.Getenv("NOTION_API_KEY")
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Notion-Version", "2022-06-28")
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("notion API returned status %d: %s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}

func pr() {
	err := godotenv.Load()
	if err != nil {
		fmt.Println("No .env file found")
	}

	pages, err := fetchNotionPages()
	if err != nil {
		fmt.Println("ERROR:", err)
		return
	}

	fmt.Println("\nNOTION PAGES:")
	for _, p := range pages {
		fmt.Printf("- %s\n", p.Title)
	}
}
