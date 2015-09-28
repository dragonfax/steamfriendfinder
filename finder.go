package sff

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
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

func getEntityRootKey(c appengine.Context) *datastore.Key {
	return datastore.NewKey(c, "Summaries", "default_summaries", 0, nil)
}

const KIND = "Summary"

func init() {
	http.HandleFunc("/", handler)
}

type PlayerSummariesResult struct {
	Response struct {
		Players []struct {
			Steamid                  int64 `json:",string"`
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
			Primaryclanid            uint64 `json:",string"`
			Timecreated              uint
			Personastateflags        uint
			Gameextrainfo            string
			Gameid                   uint `json:",string"`
			Loccountrycode           string
		}
	}
}

type StoredSummary struct {
	Steamid int64
	Online  bool
}

func getPlayerSteamIdsString() string {
	ss := make([]string, len(steamids))
	for i, s := range steamids {
		ss[i] = fmt.Sprintf("%d", s)
	}
	return strings.Join(ss, ",")
}

func getPlayerCount() int {
	return len(steamids)
}

func fetchPlayerSummaries(c appengine.Context) (*PlayerSummariesResult, error) {

	client := urlfetch.Client(c)

	url := fmt.Sprintf("http://api.steampowered.com/ISteamUser/GetPlayerSummaries/v0002/?key=%s&steamids=%s", TOKEN, getPlayerSteamIdsString())
	c.Debugf("steam API url: %v", url)
	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failure of steam API: %v", err)
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("status code was not 200 (%d)", resp.StatusCode)
	}

	buf, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		panic(err)
	}

	response := PlayerSummariesResult{}

	err = json.Unmarshal(buf, &response)
	if err != nil {
		return nil, fmt.Errorf("failure to read json %v", err)
	}

	if len(response.Response.Players) == 0 {
		return nil, fmt.Errorf("not enough players in the response %d != %d", len(response.Response.Players), getPlayerCount())
	}

	if len(response.Response.Players) > getPlayerCount() {
		return nil, fmt.Errorf("too many players in the response %d != %d", len(response.Response.Players), getPlayerCount())
	}

	return &response, nil
}

func handler(w http.ResponseWriter, r *http.Request) {

	c := appengine.NewContext(r)

	summaries, err := fetchPlayerSummaries(c)
	if err != nil {
		c.Errorf("failed to retrieve player status: %v", err)
		return
	}

	for _, playerSummary := range summaries.Response.Players {

		playerName := playerSummary.Personaname

		online := playerSummary.Gameextrainfo == "Team Fortress 2"

		if online {
			c.Debugf("%v %s is Online", playerName, time.Now())
		} else {
			c.Debugf("%v %s is Offline", playerName, time.Now())
		}

		record, err := GetRecord(c, playerSummary.Steamid)
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

func GetRecord(c appengine.Context, steamid int64) (StoredSummary, error) {

	summaries := make([]StoredSummary, 0, 1)
	_, err := datastore.NewQuery(KIND).Ancestor(getEntityRootKey(c)).Filter("__key__ =", getEntityKey(c, steamid)).GetAll(c, &summaries)
	if err != nil {
		return StoredSummary{}, err
	}

	if len(summaries) == 0 {
		return StoredSummary{Steamid: steamid, Online: false}, nil
	} else {
		return summaries[0], nil
	}
}

func getEntityKey(c appengine.Context, steamId int64) *datastore.Key {
	return datastore.NewKey(c, KIND, "", int64(steamId), getEntityRootKey(c))
}

func SaveRecord(c appengine.Context, record StoredSummary) error {
	_, err := datastore.Put(c, getEntityKey(c, record.Steamid), &record)
	return err
}
