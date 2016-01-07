package main

import (
    //"fmt"
    "time"
    "math"
    "net/http"
    s "strings"
    "regexp"
    "encoding/json"
    "github.com/ziutek/mymysql/mysql"
    _ "github.com/ziutek/mymysql/thrsafe"
)

const slaSeconds = 20

type JSONAgentInfo struct {
    User string `json:"user"`
    Status string `json:"status"`
    Conf_exten string `json:"conf_exten"`
    Seconds int `json:"seconds"`
    Campaign_id string `json:"campaign_id"`
    User_group string `json:"user_group"`
    Full_name string `json:"full_name"`
    Calls_today int `json:"calls_today"`
    Lead_id int `json:"lead_id"`
    Colour string `json:"colour"`
}

type JSONQueueInfo struct {
    Name string `json:"name"`
    Number int `json:"number"`
    Number_colour string `json:"number_colour"`
    Wait_time int `json:"wait_time"`
    Wait_time_colour string `json:"wait_time_colour"`
    Met_SLA_Percentage float64 `json:"met_sla"`
    Met_SLA_Colour string `json:"met_sla_colour"`
    Drop_SLA_Percentage float64 `json:"drop_sla"`
    Drop_SLA_Colour string `json:"drop_sla_colour"`
}

type JSONStatus struct {
    Agents map[string]JSONAgentInfo `json:"agents"`
    Queues map[string]JSONQueueInfo `json:"queues"`
}

func AgentStatusJSON() []byte {
    var output JSONStatus

    output.Agents = getAgents()
    output.Queues = getQueues()

    outputJSON, err := json.Marshal(output)
    checkErr(err)

    return outputJSON
}

func getAgents() map[string]JSONAgentInfo {
    output := make(map[string]JSONAgentInfo)

    var callerids string = ""
    cidrows, cidres, err := dbConn.Query("SELECT callerid FROM vicidial_auto_calls")
    checkErr(err)
    for _, cidrow := range cidrows {
        callerids += cidrow.Str(cidres.Map("callerid")) + "|"
    }

    rows, res, err := dbConn.Query(`
        SELECT
            extension,
            vicidial_live_agents.user,
            conf_exten,
            vicidial_live_agents.status,
            vicidial_live_agents.server_ip,
            UNIX_TIMESTAMP(last_call_time) as last_call_time,
            UNIX_TIMESTAMP(last_call_finish) as last_call_finish,
            call_server_ip,
            vicidial_live_agents.campaign_id,
            vicidial_users.user_group,
            vicidial_users.full_name,
            vicidial_live_agents.comments,
            vicidial_live_agents.calls_today,
            vicidial_live_agents.callerid,
            lead_id,
            UNIX_TIMESTAMP(last_state_change) as last_state_change,
            on_hook_agent,
            ring_callerid,
            agent_log_id
        FROM
            vicidial_live_agents,
            vicidial_users
        WHERE
            vicidial_live_agents.user = vicidial_users.user
    `)
    checkErr(err)

    for _, row := range rows {
        agent := JSONAgentInfo{}

        agent.Status = row.Str(res.Map("status"))
        if row.Str(res.Map("on_hook_agent")) == "Y" {
            agent.Status = "RING"
        }

        // 3-way check
        if row.Int(res.Map("lead_id")) != 0 {
            threeRows, _, err := dbConn.Query(`
                SELECT
                    UNIX_TIMESTAMP(last_call_time)
                FROM
                    vicidial_live_agents
                WHERE
                    lead_id = '%d'
                AND
                    status = 'INCALL'
                ORDER BY UNIX_TIMESTAMP(last_call_time) DESC
            `, row.Int(res.Map("lead_id")));
            checkErr(err)

            if len(threeRows) > 1 {
                    agent.Status = "3-WAY"
            }
        }

        pause_code := ""
        epoch_sec := 0

        if isTrue, _ := regexp.MatchString("(?i)READY|PAUSED", row.Str(res.Map("status"))); isTrue {
            epoch_sec = row.Int(res.Map("last_state_change"))

            if row.Int(res.Map("lead_id")) > 0 {
                agent.Status = "DISPO"
            } else {
                subRows, subRes, err := dbConn.Query("SELECT sub_status FROM vicidial_agent_log WHERE agent_log_id >= '%s' AND user = '%s' ORDER BY agent_log_id DESC LIMIT 1", mysql.Escape(dbConn, row.Str(res.Map("agent_log_id"))), mysql.Escape(dbConn, row.Str(res.Map("user"))))
                checkErr(err)
                if len(subRows) == 1 {
                    pause_code = subRows[0].Str(subRes.Map("sub_status"))
                }
            }
        } else {
            epoch_sec = row.Int(res.Map("last_call_time"))
        }

        if isTrue, _ := regexp.MatchString("(?i)INCALL", row.Str(res.Map("status"))); isTrue {
            incallRows, incallRes, err := dbConn.Query("SELECT UNIX_TIMESTAMP(parked_time) AS pt FROM parked_channels WHERE channel_group = '%s'", mysql.Escape(dbConn, row.Str(res.Map("callerid"))))
            checkErr(err)
            if len(incallRows) > 0 {
                agent.Status = "PARK"
                epoch_sec = incallRows[0].Int(incallRes.Map("pt"))
            } else {
                if isTrue2, _ := regexp.MatchString(regexp.QuoteMeta(row.Str(res.Map("callerid")) + "|"), callerids); !isTrue2 {
                    epoch_sec = row.Int(res.Map("last_state_change"))
                    agent.Status = "DEAD"
                }
            }
        }

        seconds := ( int(time.Now().Unix()) - epoch_sec )

        switch agent.Status {
            case "DISPO":
                agent.Colour = "8e44ad"
            case "QUEUE":
                agent.Colour = "9b59b6"
            case "INCALL":
                if row.Str(res.Map("comments")) == "INBOUND" {
                    agent.Colour = "446CB3"
                } else {
                    // Outbound call.
                    agent.Colour = "22A7F0"
                    agent.Status = "OUTBND"
                }
            case "PARK":
                agent.Colour = "E87E04"
            case "DEAD":
                agent.Colour = "004D86"
                agent.Status = "GONE"
            case "3-WAY":
                agent.Colour = "1abc9c"
            case "RING":
                agent.Colour = "16a085"
            case "PAUSED":
                switch pause_code {
                    case "TLAUTH":
                        agent.Colour = "D2527F"
                        agent.Status = "TLAUTH"
                    case "LUNCH":
                        agent.Colour = "EB974E"
                        agent.Status = "LUNCH"
                    case "BREAK":
                        agent.Colour = "96281B"
                        agent.Status = "BREAK"
                    case "COMBR":
                        agent.Colour = "F5D76E"
                        agent.Status = "COMFORT"
                    case "TRADEV":
                        agent.Colour = "E08283"
                        agent.Status = "TRADEV"
                        break;
                    case "LOGIN":
                        agent.Colour = "000000";
                        agent.Status = "LOGIN"
                    default:
                        if seconds > 300 {
                            agent.Colour = "C0392B"
                        } else {
                            agent.Colour = "F22613"
                        }
                }
            case "CLOSER":
                agent.Status = "READY (C)"
                agent.Colour = "27ae60"
            case "READY":
                agent.Colour = "27ae60"
            default:
                agent.Colour = "D2BEAA"
        }

        agent.User = row.Str(res.Map("user"))
        agent.Conf_exten = row.Str(res.Map("conf_exten"))
        agent.Seconds = seconds
        agent.Campaign_id = row.Str(res.Map("campaign_id"))
        agent.User_group = row.Str(res.Map("user_group"))
        agent.Full_name = row.Str(res.Map("full_name"))
        agent.Calls_today = row.Int(res.Map("calls_today"))
        agent.Lead_id = row.Int(res.Map("lead_id"))

        output[row.Str(res.Map("extension"))] = agent
    }

    return output
}

func getQueues() map[string]JSONQueueInfo {
    output := make(map[string]JSONQueueInfo)

    start_time := int(time.Now().Unix())

    output["global"] = getGlobalQueue(start_time)

    campaignRows, campaignRes, err := dbConn.Query(`
        SELECT
            campaign_id,
            campaign_name,
            closer_campaigns
        FROM
            vicidial_campaigns
        WHERE
            active='Y'
    `)
    checkErr(err)

    for _, campaignRow := range campaignRows {
        queue := JSONQueueInfo{}
        queue.Name = campaignRow.Str(campaignRes.Map("campaign_name"))
        inbound_list := s.Split(mysql.Escape(dbConn, s.TrimSpace(s.Replace(campaignRow.Str(campaignRes.Map("closer_campaigns")), "-", "", -1))), " ")

        callRows, callRes, err := dbConn.Query(`
                SELECT
                    status,
                    campaign_id,
                    phone_number,
                    server_ip,
                    UNIX_TIMESTAMP(call_time) as call_time,
                    call_type,
                    queue_priority,
                    agent_only
                FROM
                    vicidial_auto_calls
                WHERE
                        status IN ('LIVE')
                    AND
                        call_type='IN'
                    AND
                        campaign_id IN ('` + s.Join(inbound_list, "', '") + `')
                ORDER BY
                    call_time ASC
        `)
        checkErr(err)

        if len(callRows) > 0 {
            queue.Number = len(callRows)
            queue.Wait_time = ( start_time - callRows[0].Int(callRes.Map("call_time")))
        } else {
            queue.Number = 0
            queue.Wait_time = 0
        }
        queue.Number_colour = number_colour(queue.Number)
        queue.Wait_time_colour = wait_time_colour(queue.Wait_time)

        tFormat := "2006-01-02"
        tNow := time.Now()
        tBegin := tNow.Format(tFormat) + " 00:00:00"
        tEnd := tNow.Format(tFormat) + " 23:59:59"
        var slaTotal float64 = 0

        slaTotalRows, slaTotalRes, err := dbConn.Query(`
            SELECT
                COUNT(*) as 'total'
            FROM
                vicidial_closer_log
            WHERE
                    status NOT IN ( 'INCALL', 'AFTHRS' )
                AND
                    call_date BETWEEN '%s' AND '%s'
                AND
                    campaign_id IN ('` + s.Join(inbound_list, "', '") + `')
        `, mysql.Escape(dbConn, tBegin), mysql.Escape(dbConn, tEnd))
        checkErr(err)
        if len(slaTotalRows) > 0 {
            slaTotal = float64(slaTotalRows[0].Int(slaTotalRes.Map("total")))
        } else {
            slaTotal = 0
        }

        if slaTotal > 0 {
            slaMetRows, slaMetRes, err := dbConn.Query(`
                SELECT
                    COUNT(*) as 'total'
                FROM
                    vicidial_closer_log
                WHERE
                        status NOT IN ( 'INCALL', 'AFTHRS' )
                    AND
                        call_date BETWEEN '%s' AND '%s'
                    AND
                        queue_seconds < %d
                    AND
                        campaign_id IN ('` + s.Join(inbound_list, "', '") + `')
            `, mysql.Escape(dbConn, tBegin), mysql.Escape(dbConn, tEnd), slaSeconds)
            checkErr(err)
            if len(slaMetRows) > 0 {
                queue.Met_SLA_Percentage = Round( (( float64(slaMetRows[0].Int(slaMetRes.Map("total"))) / slaTotal ) * 100), .5, 0 )
            } else {
                queue.Met_SLA_Percentage = 0
            }

            slaDropRows, slaDropRes, err := dbConn.Query(`
                SELECT
                    COUNT(*) as 'total'
                FROM
                    vicidial_closer_log
                WHERE
                        status = 'DROP'
                    AND
                        call_date BETWEEN '%s' AND '%s'
                    AND
                        campaign_id IN ('` + s.Join(inbound_list, "', '") + `')
            `, mysql.Escape(dbConn, tBegin), mysql.Escape(dbConn, tEnd))
            checkErr(err)
            if len(slaDropRows) > 0 {
                queue.Drop_SLA_Percentage = Round( (( float64(slaDropRows[0].Int(slaDropRes.Map("total"))) / slaTotal ) * 100), .5, 0 )
            } else {
                queue.Drop_SLA_Percentage = 0
            }
        } else {
            queue.Met_SLA_Percentage = 100
            queue.Drop_SLA_Percentage = 0
        }
        queue.Met_SLA_Colour = sla_met_colour(queue.Met_SLA_Percentage)
        queue.Drop_SLA_Colour = sla_drop_colour(queue.Drop_SLA_Percentage)

        output[campaignRow.Str(campaignRes.Map("campaign_id"))] = queue
    }

    return output
}

func getGlobalQueue(start_time int) JSONQueueInfo {
    globalRows, globalRes, err := dbConn.Query(`
        SELECT
            status,
            campaign_id,
            phone_number,
            server_ip,
            UNIX_TIMESTAMP(call_time) as call_time,
            call_type,
            queue_priority,
            agent_only
        FROM
            vicidial_auto_calls
        WHERE
                status IN ('LIVE')
            AND
                call_type='IN'
        ORDER BY
            call_time ASC
    `)
    checkErr(err)

    globalQueue := JSONQueueInfo{}
    globalQueue.Name = "Global Queue Name"
    if len(globalRows) > 0 {
        globalQueue.Number = len(globalRows)
        globalQueue.Wait_time = ( start_time - globalRows[0].Int(globalRes.Map("call_time")) )
    } else {
        globalQueue.Number = 0
        globalQueue.Wait_time = 0
    }
    globalQueue.Number_colour = number_colour(globalQueue.Number)
    globalQueue.Wait_time_colour = wait_time_colour(globalQueue.Wait_time)

    tFormat := "2006-01-02"
    tNow := time.Now()
    tBegin := tNow.Format(tFormat) + " 00:00:00"
    tEnd := tNow.Format(tFormat) + " 23:59:59"
    var slaTotal float64 = 0

    slaTotalRows, slaTotalRes, err := dbConn.Query(`
        SELECT
            COUNT(*) as 'total'
        FROM
            vicidial_closer_log
        WHERE
                status NOT IN ( 'INCALL', 'AFTHRS' )
            AND
                call_date BETWEEN '%s' AND '%s'
    `, mysql.Escape(dbConn, tBegin), mysql.Escape(dbConn, tEnd))
    checkErr(err)
    if len(slaTotalRows) > 0 {
        slaTotal = float64(slaTotalRows[0].Int(slaTotalRes.Map("total")))
    } else {
        slaTotal = 0
    }

    if slaTotal > 0 {
        slaMetRows, slaMetRes, err := dbConn.Query(`
            SELECT
                COUNT(*) as 'total'
            FROM
                vicidial_closer_log
            WHERE
                    status NOT IN ( 'INCALL', 'AFTHRS' )
                AND
                    call_date BETWEEN '%s' AND '%s'
                AND
                    queue_seconds < %d
        `, mysql.Escape(dbConn, tBegin), mysql.Escape(dbConn, tEnd), slaSeconds)
        checkErr(err)
        if len(slaMetRows) > 0 {
            globalQueue.Met_SLA_Percentage = Round( (( float64(slaMetRows[0].Int(slaMetRes.Map("total"))) / slaTotal ) * 100), .5, 0 )
        } else {
            globalQueue.Met_SLA_Percentage = 0
        }

        slaDropRows, slaDropRes, err := dbConn.Query(`
            SELECT
                COUNT(*) as 'total'
            FROM
                vicidial_closer_log
            WHERE
                    status = 'DROP'
                AND
                    call_date BETWEEN '%s' AND '%s'
        `, mysql.Escape(dbConn, tBegin), mysql.Escape(dbConn, tEnd))
        checkErr(err)
        if len(slaDropRows) > 0 {
            globalQueue.Drop_SLA_Percentage = Round( (( float64(slaDropRows[0].Int(slaDropRes.Map("total"))) / slaTotal ) * 100), .5, 0 )
        } else {
            globalQueue.Drop_SLA_Percentage = 0
        }
    } else {
        globalQueue.Met_SLA_Percentage = 100
        globalQueue.Drop_SLA_Percentage = 0
    }
    globalQueue.Met_SLA_Colour = sla_met_colour(globalQueue.Met_SLA_Percentage)
    globalQueue.Drop_SLA_Colour = sla_drop_colour(globalQueue.Drop_SLA_Percentage)

    return globalQueue
}

func number_colour(number int) string {
    if (number < 1){
		return "27ae60"
	} else if (number < 5){
		return "e67e22"
	} else if (number < 15){
		return "e74c3c"
	} else {
		return "c0392b"
	}
}

func wait_time_colour(time int) string {
    if (time < 20){
		return "27ae60"
	} else if (time < 120) {
		return "e67e22"
	} else if (time < 300) {
		return "e74c3c"
	} else {
		return "c0392b"
	}
}

func sla_met_colour(number float64) string {
    if (number >= 80){
        return "27ae60"
    } else if (number >= 75) {
        return "e67e22"
    } else if (number >= 70) {
        return "e74c3c"
    } else {
        return "c0392b"
    }
}

func sla_drop_colour(number float64) string {
    if (number < 5){
        return "27ae60"
    } else if (number < 15) {
        return "e67e22"
    } else if (number < 25) {
        return "e74c3c"
    } else {
        return "c0392b"
    }
}

func Round(val float64, roundOn float64, places int ) (newVal float64) {
	var round float64
	pow := math.Pow(10, float64(places))
	digit := pow * val
	_, div := math.Modf(digit)
	if div >= roundOn {
		round = math.Ceil(digit)
	} else {
		round = math.Floor(digit)
	}
	newVal = round / pow
	return
}

func AgentStatusSingle(w http.ResponseWriter, r *http.Request) {
    w.Write(AgentStatusJSON())
}
