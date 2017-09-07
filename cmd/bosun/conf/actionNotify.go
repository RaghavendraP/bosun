package conf

import (
	"bytes"
	"fmt"
	"net/url"
	"strings"

	"bosun.org/slog"

	"bosun.org/cmd/bosun/conf/template"
	"bosun.org/models"
)

var defaultActionNotificationSubjectTemplate *template.Template
var defaultActionNotificationBodyTemplate *template.Template

func init() {
	subject := `{{$first := index .States 0}}{{$count := len .States}}
{{.User}} {{.ActionType}}
{{if gt $count 1}} {{$count}} Alerts. 
{{else}} Incident #{{$first.Id}} ({{$first.Subject}}) 
{{end}}`
	body := `{{$count := len .States}}{{.User}} {{.ActionType}} {{$count}} alert{{if gt $count 1}}s{{end}}: <br/>
<strong>Message:</strong> {{.Message}} <br/>
<strong>Incidents:</strong> <br/>
<ul>
	{{range .States}}
		<li>
			<a href="{{$.IncidentLink .Id}}">#{{.Id}}:</a> 
			{{.Subject}}
		</li>
	{{end}}
</ul>`
	defaultActionNotificationSubjectTemplate = template.Must(template.New("subject").Parse(strings.Replace(subject, "\n", "", -1)))
	defaultActionNotificationBodyTemplate = template.Must(template.New("body").Parse(body))
}

type actionNotificationContext struct {
	States     []*models.IncidentState
	User       string
	Message    string
	ActionType models.ActionType
	makeLink   func(string, *url.Values) string
}

func (a actionNotificationContext) IncidentLink(i int64) string {
	return a.makeLink("/incident", &url.Values{
		"id": []string{fmt.Sprint(i)},
	})
}

// NotifyAction should be used for action notifications.
func (n *Notification) NotifyAction(at models.ActionType, t *Template, c SystemConfProvider, states []*models.IncidentState, user, message string) {
	n.PrepareAction(at, t, c, states, user, message).Send(c)
}

func (n *Notification) RunOnActionType(at models.ActionType) bool {
	if n.RunOnActions == "all" || n.RunOnActions == "true" {
		return true
	}
	if n.RunOnActions == "none" || n.RunOnActions == "false" {
		return false
	}
	for _, a := range strings.Split(n.RunOnActions, ",") {
		if models.ActionShortNames[a] == at {
			return true
		}
	}
	return false
}

// Prepate an action notification, but don't send yet.
func (n *Notification) PrepareAction(at models.ActionType, t *Template, c SystemConfProvider, states []*models.IncidentState, user, message string) *PreparedNotifications {
	pn := &PreparedNotifications{}
	// get template keys to use for things. Merge with default sets
	tks := n.ActionTemplateKeys[at].Combine(n.ActionTemplateKeys[models.ActionNone])
	buf := &bytes.Buffer{}
	render := func(key string, defaultTmpl *template.Template) (string, error) {
		tpl := defaultTmpl
		if key != "" {
			tpl = t.Get(key)
		} else {
			key = "default"
		}
		buf.Reset()
		ctx := actionNotificationContext{
			States:     states,
			User:       user,
			Message:    message,
			ActionType: at,
			makeLink:   c.MakeLink,
		}
		err := tpl.Execute(buf, ctx)
		if err != nil {
			e := fmt.Sprintf("executing action template '%s': %s", key, err)
			pn.Errors = append(pn.Errors, e)
			slog.Errorf(e)
			return "", err
		}
		return buf.String(), nil
	}
	n.prepareFromTemplateKeys(pn, *tks, render, defaultActionNotificationSubjectTemplate, defaultActionNotificationBodyTemplate)
	return pn
}