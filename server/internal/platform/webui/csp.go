package webui

// contentSecurityPolicy is the CSP of the app's pages. Scripts come from
// nervewiki only: the built index.html has no inline script (the theme is
// set before the first paint by the file theme-init.js). Styles may be
// inline, for the style attributes that React and Radix's popovers set;
// nothing else is loaded from anywhere but nervewiki, and no other site may
// frame the pages.
const contentSecurityPolicy = "default-src 'self'; " +
	"script-src 'self'; " +
	"style-src 'self' 'unsafe-inline'; " +
	"img-src 'self' data: blob:; " +
	"font-src 'self' data:; " +
	"connect-src 'self'; " +
	"object-src 'none'; " +
	"base-uri 'none'; " +
	"form-action 'self'; " +
	"frame-ancestors 'none'"
