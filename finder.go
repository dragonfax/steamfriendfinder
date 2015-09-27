package sff

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"appengine"
	"appengine/datastore"
	"appengine/mail"
	"appengine/urlfetch"
)

/*
$ curl 'http://api.steampowered.com/ISteamUser/GetPlayerSummaries/v0002/?key=XXXXXX&steamids=76561197970839813'
{
	"response": {
		"players": [
			{
				"steamid": "76561197970839813",
				"communityvisibilitystate": 3,
				"profilestate": 1,
				"personaname": "H311B0Y",
				"lastlogoff": 1443115849,
				"profileurl": "http://steamcommunity.com/profiles/76561197970839813/",
				"avatar": "https://steamcdn-a.akamaihd.net/steamcommunity/public/images/avatars/06/060118081184c4b75b91cd0c2864b01414b26e09.jpg",
				"avatarmedium": "https://steamcdn-a.akamaihd.net/steamcommunity/public/images/avatars/06/060118081184c4b75b91cd0c2864b01414b26e09_medium.jpg",
				"avatarfull": "https://steamcdn-a.akamaihd.net/steamcommunity/public/images/avatars/06/060118081184c4b75b91cd0c2864b01414b26e09_full.jpg",
				"personastate": 3,
				"realname": "Chris Barnard",
				"primaryclanid": "103582791432065012",
				"timecreated": 1100920678,
				"personastateflags": 0,
				"gameextrainfo": "Team Fortress 2",
				"gameid": "440",
				"loccountrycode": "US"
			}
		]
	}
*/

func getEntityKey(c appengine.Context) *datastore.Key {
	return datastore.NewKey(c, "Summaries", "default_summaries", 0, nil)
}

const KIND = "Summary"

func init() {
	http.HandleFunc("/", handler)
}

type Summary struct {
	Steamid int64
	Online  bool
}

func handler(w http.ResponseWriter, r *http.Request) {

	c := appengine.NewContext(r)

	client := urlfetch.Client(c)

	for _, steamid := range steamids {

		url := fmt.Sprintf("http://api.steampowered.com/ISteamUser/GetPlayerSummaries/v0002/?key=%s&steamids=%d", TOKEN, steamid)
		resp, err := client.Get(url)
		if err != nil {
			c.Errorf("failure of steam API: %v", err)
			return
		}

		if resp.StatusCode != 200 {
			c.Errorf("status code was not 200 (%d)", resp.StatusCode)
			return
		}

		buf, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			panic(err)
			return
		}

		response := struct {
			Response struct {
				Players []struct {
					Steamid                  int64 `json:"string"`
					Communityvisibilitystate uint
					Profilestate             uint
					Personaname              string
					Lastlogoff               uint
					Profileurl               string
					Avatar                   string
					Avatarmedium             string
					Avatarfull               string
					Personastate             uint
					Realname                 string
					Primaryclanid            uint64 `json:"string"`
					Timecreated              uint
					Personastateflags        uint
					Gameextrainfo            string
					Gameid                   uint `json:"string"`
					Loccountrycode           string
				}
			}
		}{}

		err = json.Unmarshal(buf, &response)
		if err != nil {
			c.Errorf("failure to read json %v", err)
			return
		}

		if len(response.Response.Players) == 0 {
			c.Errorf("not enough players in the response")
			return
		}

		if len(response.Response.Players) > 1 {
			c.Errorf("too many players in the response")
			return
		}

		playerName := response.Response.Players[0].Personaname

		online := response.Response.Players[0].Gameextrainfo == "Team Fortress 2"

		if online {
			c.Debugf("%v %s is Online", playerName, time.Now())
		} else {
			c.Debugf("%v %s is Offline", playerName, time.Now())
		}

		record, err := GetRecord(c, steamid)
		if err != nil {
			c.Errorf("failure to get record %v", err)
			return
		}

		newlyOnline := online && !record.Online
		newlyOffline := !online && record.Online

		if newlyOnline || newlyOffline {
			record.Online = online
			err = SaveRecord(c, record)
			if err != nil {
				c.Errorf("failure to save record %v", err)
				return
			}
		}

		if newlyOnline {
			msg := &mail.Message{
				Sender:  "admin@steamfriendfinder.appspotmail.com",
				Subject: fmt.Sprintf("%s is playing Team Fortress 2", playerName),
			}
			if err := mail.SendToAdmins(c, msg); err != nil {
				c.Errorf("Couldn't send email: %v", err)
				return
			}
			c.Debugf("Sending email")
		}

	}
}

func GetRecord(c appengine.Context, steamid int64) (Summary, error) {

	summaries := make([]Summary, 0, 1)
	_, err := datastore.NewQuery(KIND).Ancestor(getEntityKey(c)).Filter("Steamid =", steamid).GetAll(c, &summaries)
	if err != nil {
		return Summary{}, err
	}

	if len(summaries) == 0 {
		return Summary{Steamid: steamid, Online: false}, nil
	} else {
		return summaries[0], nil
	}
}

func SaveRecord(c appengine.Context, record Summary) error {

	_, err := datastore.Put(c, datastore.NewIncompleteKey(c, KIND, getEntityKey(c)), &record)

	return err
}
