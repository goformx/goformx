---
title: Embedding Forms
order: 3
---

# Embedding Forms

You can embed any GoFormX form on your website using an iframe or by linking directly.

## Direct link

Every published form is accessible at:

```
https://goformx.com/forms/{form-id}
```

Share this URL via email, social media, or anywhere else.

## Embed with iframe

Add this HTML to your page:

```html
<iframe
  src="https://goformx.com/forms/{form-id}/embed"
  width="100%"
  height="600"
  frameborder="0"
  style="border: none;"
></iframe>
```

The embed endpoint renders the form without the site header and footer for a clean embedded experience.

## Styling tips

- Set `width="100%"` and a fixed height, or use CSS to make the iframe responsive
- The form inherits its own styling — it won't conflict with your site's CSS
- For dark backgrounds, the form supports dark mode automatically
