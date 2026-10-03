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

// workerPolicy is the CSP of the scripts under assets/. A script a page
// loads runs under the page's policy, its own ignored; one that runs as a
// worker runs under the policy of its own answer (CSP 3, 4.2.1): the
// reading view's highlighting, given the pages' contents, then loads and
// sends nothing (M4/P5 review m8). A worker that imports another module
// (import(), importScripts) needs that allowed here first.
const workerPolicy = "default-src 'none'"
