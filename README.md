# Vicidial Agent Wallboard

This is the HTML5 / GoLang / WebSocket version of our Vicidial agent wallboard.

Tested with Go v1.5.1

## WebSockets are the future!

Our wallboard became very popular, on loads of screens and desktops all around the place! But for a while, that was it's own demise.

Using the old method of every instance polling a PHP script every second for updates, which would individually run all the required queries, it was quickly locking up the database server as the number of users increased.

As a soloution, this new method runs the queries only once, then sends them out to every connected user via their persistant WebSocket connection.

Plus, as an added bonus, everything looks a lot smoother and runs more fluidly! :).

## Features

* WebSockets by default (single set of queries to all users).
* Option for polling (each user runs their own set of queries, like the old wallboard).
* Inbound SLA & Drop rates in the main header (globally or per campaign, same as for the wait times).

## How to run...

* Edit files for DB config (main.go), SLA time (agent_status.go).
* Make sure you're in the folder, then `go build` should produce an executable named `pbx_wallboard`
* Copy `pbx_wallboard` plus the folders `wallboard_html` & `wallboard_templates` into a folder on your server.
* Place `data.json` from your old wallboard install in the same folder (generating this not currently supported).
* Run `pbx_wallboard` in a screen, detach and leave running!
* You can access via:
  * WebSocket version: `http://<server>:8888/wallboard/`
  * Polling version: `http://<server>:8888/wallboard/poll.html`
  * Status JSON: `http://<server>:8888/wallboard/status`
  * Edit Layout: `http://<server>:8888/wallboard/edit.html`
    * Beware this layout editor is unauthenticated and will allow arbitrary post data to be written to data.json
    * It is advisable to secure both this URL and the save endpoint `http://<server>:8888/wallboard/save` with some kind of auth / restriction.
* We use nginx to reverse proxy to ours, for SSL & if running two instances on different ports, you can have a backup.
