package main

import (
    "net/http"
    "github.com/gorilla/mux"
    "github.com/codegangsta/negroni"
    "github.com/ziutek/mymysql/mysql"
    _ "github.com/ziutek/mymysql/thrsafe"
)

var dbConn mysql.Conn

func IndexHandler(w http.ResponseWriter, r *http.Request) {
    // Set the location header to redirect to admin panel.
    w.Write([]byte("Permission Denied"))
    // Write the HTTP response code. This has to be written after any required headers are set.
    w.WriteHeader(403)
}

func main() {
    dbConn = mysql.New("tcp", "", "127.0.0.1:3306", "cron", "1234", "asterisk")
    err := dbConn.Connect()
    checkErr(err)

    go sendLatestJSON()

    r := mux.NewRouter()
    // Routes consist of a path and a handler function.
    r.HandleFunc("/", IndexHandler)

    wallboard_base := mux.NewRouter()
    r.PathPrefix("/wallboard").Handler(negroni.New(
        negroni.NewRecovery(),
        negroni.NewLogger(),
        &negroni.Static{
            Dir: http.Dir("wallboard_html"),
            Prefix: "/wallboard",
            IndexFile: "index.html",
        },
        negroni.Wrap(wallboard_base),
    ))
    wallboard_sub := wallboard_base.PathPrefix("/wallboard").Subrouter()
    // Wallboard WebSocket Page
    wallboard_sub.HandleFunc("/",               WallboardPage)
    wallboard_sub.HandleFunc("/index.html",     WallboardPage)
    wallboard_sub.HandleFunc("/wallboard.php",  WallboardPage)
    // Old style polling interface
    wallboard_sub.HandleFunc("/poll.html",      WallboardPollPage)
    // Edit page.
    wallboard_sub.HandleFunc("/edit.html",      WallboardEditPage)
    wallboard_sub.HandleFunc("/save",           WallboardSavePage)
    // WebSocket Interface
    wallboard_sub.HandleFunc("/status_ws",      AgentStatusWS)
    // Polling JSON Interface
    wallboard_sub.HandleFunc("/status",         AgentStatusSingle)
    //wallboard_sub.HandleFunc("/wallboard_json.php", AgentStatusSingle)

    // Bind to a port and pass our router in
    http.ListenAndServe(":8888", r)
}

func checkErr(err error) {
    if err != nil {
        panic(err)
    }
}
