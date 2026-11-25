// site_open(path): a page of the site — in this tab when the game plays in
// the site's own page (/ludo/), else the site in the browser.
if (global.site) url_open_ext(argument0, "_self");
else url_open("https://esperanto-kurso.net" + argument0);
