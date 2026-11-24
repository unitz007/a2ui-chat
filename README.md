# a2ui-chat

Render [A2UI](https://a2ui.org) surfaces in **Slack, Discord and Telegram**, with each app's own widgets, and turn
what people do back into A2UI action messages for your agent.

A2UI lets an agent describe an interface (cards, lists, choices, forms, buttons) as declarative JSON, and leaves
drawing it to the client. There are renderers for web and mobile frameworks; this is one for chat apps. An agent that
speaks A2UI can show the same surface in a web app and in a Slack, Discord or Telegram chat.

```
agent ──A2UI JSONL──▶ a2ui.Surfaces ──▶ slack / discord / telegram renderer ──▶ your bot sends it
agent ◀──A2UI action── chatui.HandleEvent ◀── renderer.Interaction ◀── the app posts a tap or a form
```

Status: **v0.1, early.** It implements the A2UI v1.0 basic catalog. It is tested against recorded app payloads, not yet
against live Slack, Discord or Telegram workspaces. The API may change.

## Install

```sh
go get github.com/unitz007/a2ui-chat
```

## Use

```go
ss := a2ui.NewSurfaces()
a2ui.ReadStream(agentOutput, func(m *a2ui.Message) error {
	_, err := ss.Apply(m)
	return err
})
s, _ := ss.Get("pick-task")

// Slack: blocks for chat.postMessage (text is the plain words, for notifications).
blocks, text, err := slack.Renderer{}.Message(s)

// Discord: components, sent with discord.Flags (its newer message layout) and no content.
components, err := discord.Renderer{Codec: codec}.Message(s)

// Telegram: HTML text and an inline keyboard.
msg, err := telegram.Renderer{Codec: codec}.Message(s)
```

When someone taps a button, picks an option or ticks a box, hand what the app posted to the renderer, then apply it:

```go
ev, form, ok, err := slack.Renderer{}.Interaction(payloadJSON)
if !ok {
	return // not one of ours
}
if ev != nil {
	res, err := chatui.HandleEvent(ss, *ev)
	switch {
	case res.Action != nil: // a button: send it to the agent
		send(a2ui.NewAction(res.Action))
	case res.Redraw: // a choice or a tick: draw the message again (chat.update)
	case res.OpenForm: // Edit…: open the form (views.open with Renderer.Modal)
	}
}
if form != nil { // a submitted form
	bad, err := chatui.Submit(ss, form.Surface, form.Values, form.Scopes)
	respond(form.Errors(bad)) // Slack shows each problem under its field
}
```

The library makes no network calls and holds no tokens: it builds what to send and reads what comes back. Your bot
does the sending.

## How each app shows a surface

| A2UI | Slack | Discord | Telegram |
|---|---|---|---|
| Text | section (caption: context; h1–h5 bold) | text display (caption: `-#`; h1–h5 bold) | HTML text (h1–h5 bold) |
| Text's Markdown: **bold**, *italic*, `code`, [links](https://…), `> ` quoted lines | mrkdwn (`*b*`, `_i_`, `<url\|words>`, `>`) | Discord markdown | `<b>`, `<i>`, `<code>`, `<a>`, `<blockquote>` |
| Card | its parts in order | container, with an accent colour | its parts in order |
| Row, Column, List (with templates), Tabs, Modal | flattened in order | flattened in order | flattened in order |
| Image | image block | media gallery | link |
| Divider | divider | separator | blank line |
| Button | button (primary style) | button (primary style); link button for `openUrl` | inline button (✅ for primary) |
| ChoicePicker | radio buttons, or a menu past 10 options; tick boxes or a multi-select for several | select menu (several: `max_values`) | a button per option, the chosen one marked |
| CheckBox | tick box | button showing ☐ / ☑ | button showing ☐ / ☑ |
| TextField, Slider, DateTimeInput | values in the message, and **Edit…** opens a Slack modal (text, number, date, time inputs) | values in the message, and **Edit…** opens a Discord modal (text inputs, five at most) | values in the message, and **Edit…** opens a Mini App you serve (`Renderer.MiniApp`); none in groups |
| `@checks` | shown under the input; a modal's errors via `Form.Errors` | shown under the input | shown in the text |

Values bound to the data model (`{"@path": …}`) are read when drawing. Inputs write back to their bound path (two-way
binding), and a button's action context is read when it is pressed, as the spec says. `formatString`, `formatNumber`,
`formatCurrency`, `formatDate`, `pluralize` and the validation functions (`required`, `regex`, `length`, `numeric`,
`email`, `and`, `or`, `not`) are supported. `openUrl` becomes a link button.

## Ids on buttons

Each app carries an id back with a tap: Slack's `action_id` (255 characters), Discord's `custom_id` (100) and
Telegram's `callback_data` (64 bytes). A `chatui.Codec` turns a target (surface, component, scope, option) into that
id:

- `chatui.Inline` carries it in the id itself. Nothing is stored, but ids are long. It's the Slack renderer's default.
- `chatui.Memory` keeps targets in memory under short ids. It's the Telegram renderer's default, and fine for one
  server.
- Several servers need targets kept in a shared store: implement `Codec` on your own database.

Surfaces live in `a2ui.Surfaces` in memory. A host with several servers keeps each surface's components and data
(`Surface.Components`, `Surface.Data`) in its own store, and rebuilds them with `Surfaces.Put`.

## Not done yet

- Renderer and agent function calls (`callRendererFunction`, `callAgentFunction`): carried, not run.
- Slack's table block, Discord's newer modal layout, Telegram's native inputs.
- Live tests against each app.

## Licence

Apache 2.0, the same as A2UI. Not affiliated with the A2UI project or Google.
