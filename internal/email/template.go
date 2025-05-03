package email

import (
	"bytes"
	"fmt"
	"html/template"
	"sort"
	"strings"
)

const signatureHTML = `
<p>Una abraçada!</p>
<p>🌚 ZenithPlanner bot</p>
`

// formatGenericSyncEmail formats the email summary for full sync changes using HTML.
func (c *Client) formatGenericSyncEmail(changes map[string]string) (string, string, error) {
	subject := "[ZenithPlanner] 💺 Location changed successfully"

	// Sort dates for consistent output
	dates := make([]string, 0, len(changes))
	for dateStr := range changes {
		dates = append(dates, dateStr)
	}
	sort.Strings(dates)

	var changeLines []string
	for _, dateStr := range dates {
		changeLines = append(changeLines, fmt.Sprintf("<li><strong>%s</strong>: %s</li>", dateStr, template.HTMLEscapeString(changes[dateStr])))
	}

	bodyData := map[string]interface{}{
		"ChangeList": template.HTML("<ul>" + strings.Join(changeLines, "") + "</ul>"), // Mark as HTML
		"Signature":  template.HTML(signatureHTML),                                    // Mark as HTML
	}

	bodyTmpl := `
	<p>Hi,</p>
	<p>You have successfully changed your location for the following dates:</p>
	{{.ChangeList}}
	{{.Signature}}
	`
	t, err := template.New("fullsync").Parse(bodyTmpl)
	if err != nil {
		return "", "", fmt.Errorf("failed to parse full sync template: %w", err)
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, bodyData); err != nil {
		return "", "", fmt.Errorf("failed to execute full sync template: %w", err)
	}

	return subject, buf.String(), nil
}
