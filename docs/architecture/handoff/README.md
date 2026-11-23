# Ask AI side panel

Drop-in panel for the Omnipro Docs site. Opens from any `[data-ask]` button or ⌘I; Esc closes.

## Install
1. Copy `ask-ai.css` and `ask-ai.js` into `public/`.
2. In `index.html`:
```html
<link rel="stylesheet" href="/ask-ai.css">
<script src="/ask-ai.js" data-logo="/assets/blusalt-icon.svg" defer></script>
```
Uses existing tokens from styles.css (--blue, --blue-light, --line, --text, --muted, --bg); light and dark themes supported.

## Notes
- Answers are canned for the demo — replace the answer function in ask-ai.js with the real API call.
- "Thinking" state spins the logo (2.6s/turn; slower under reduced motion).
- Pushes content on screens ≥1100px, overlays below that.
