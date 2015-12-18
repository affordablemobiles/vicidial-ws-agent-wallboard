package main

import (
    "net/http"
    "html/template"
    "encoding/json"
    "io/ioutil"
    "strconv"
    //"github.com/ziutek/mymysql/mysql"
    //_ "github.com/ziutek/mymysql/thrsafe"
)

type HTMLWallboardData struct {
    Edit bool
    Poll bool
    Campaigns map[string]string
    Boxes map[string]map[string]int
}

func WallboardPage(w http.ResponseWriter, r *http.Request) {
    t, err := template.ParseFiles("wallboard_templates/index.html")
    checkErr(err)

    data := getBasicData()

    t.Execute(w, data)
}

func WallboardPollPage(w http.ResponseWriter, r *http.Request) {
    t, err := template.ParseFiles("wallboard_templates/index.html")
    checkErr(err)

    data := getBasicData()
    data.Poll = true

    t.Execute(w, data)
}

func WallboardEditPage(w http.ResponseWriter, r *http.Request) {
    t, err := template.ParseFiles("wallboard_templates/index.html")
    checkErr(err)

    data := getBasicData()
    data.Edit = true

    t.Execute(w, data)
}

func getBasicData() HTMLWallboardData {
    output := HTMLWallboardData{ Edit: false, Poll: false }

    output.Campaigns = make(map[string]string)
    campaignRows, campaignRes, err := dbConn.Query(`
        SELECT
            campaign_id,
            campaign_name
        FROM
            vicidial_campaigns
        WHERE
            active='Y'
    `)
    checkErr(err)
    for _, campaignRow := range campaignRows {
        output.Campaigns[campaignRow.Str(campaignRes.Map("campaign_id"))] = campaignRow.Str(campaignRes.Map("campaign_name"))
    }

    output.Boxes = make(map[string]map[string]int)

    data, err := ioutil.ReadFile("wallboard_html/data.json")
    checkErr(err)
    var m map[string]map[string]string
    err = json.Unmarshal(data, &m)
    checkErr(err)
    for name, value := range m {
        values := make(map[string]int)
        for k, v := range value {
            values[k], _ = strconv.Atoi(v)
        }
        output.Boxes[name] = values
    }

    return output
}

func WallboardSavePage(w http.ResponseWriter, r *http.Request) {
    //fmt.Fprintf(w, "%s", AgentStatusJSON())
}
