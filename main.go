package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"go.uber.org/zap"
	"google.golang.org/appengine/mail"
	"google.golang.org/appengine/urlfetch"
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
				"personaname": "SuperCoolBoy",
				"lastlogoff": 1443115849,
				"profileurl": "http://steamcommunity.com/profiles/76561197970839813/",
				"avatar": "https://steamcdn-a.akamaihd.net/steamcommunity/public/images/avatars/06/060118081184c4b75b91cd0c2864b01414b26e09.jpg",
				"avatarmedium": "https://steamcdn-a.akamaihd.net/steamcommunity/public/images/avatars/06/060118081184c4b75b91cd0c2864b01414b26e09_medium.jpg",
				"avatarfull": "https://steamcdn-a.akamaihd.net/steamcommunity/public/images/avatars/06/060118081184c4b75b91cd0c2864b01414b26e09_full.jpg",
				"personastate": 3,
				"realname": "Some Fellow",
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

var awsSess = session.Must(session.NewSession())
var dyndb = dynamodb.New(awsSess)

var logger, _ = zap.NewProduction()

type FriendSummary struct {
	Steamid                  int64
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
	Timecreated              uint
	Personastateflags        uint
	Gameextrainfo            string
	Gameid                   uint
}

type PlayerSummariesResult struct {
	Response struct {
		Players []*PlayerSummariesResult
	}
}

func getPlayerSteamIdsString() string {
	ss := make([]string, len(steamids))
	for i, s := range steamids {
		ss[i] = fmt.Sprintf("%d", s)
	}
	return strings.Join(ss, ",")
}

func fetchPlayerSummaries() (*PlayerSummariesResult, error) {

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

func handler(event Event) {

	if isCronEvent {
		handleCronEvent()
	} else {
		handleSQSEvent(event)
	}
}

func handleCronEvent() {

	var err error
	var summaries *PlayerSummariesResult
RETRY:
	for i := 0; i < 3; i++ {
		summaries, err = fetchPlayerSummaries(c)
		if err == nil {
			break RETRY
		}
	}
	if err != nil {
		logger.Error("failed to retrieve player status: %v", zap.Error(err))
		return
	}

	for _, playerSummary := range summaries.Response.Players {

		playerName := playerSummary.Personaname

		online := playerSummary.Gameextrainfo == "Team Fortress 2"

		if online {
			logger.Debug("friend is Online", zap.String("player", playerName))
		} else {
			logger.Debug("friend is Offline", zap.String("player", playerName))
		}

		record, err := GetRecord(c, playerSummary.Steamid)
		if err != nil {
			logger.Error("failure to get record", zap.Error(err))
			return
		}

		newlyOnline := online && !record.Online
		newlyOffline := !online && record.Online

		if newlyOnline || newlyOffline {
			record.Online = online
			err = SaveRecord(c, record)
			if err != nil {
				logger.Error("failure to save record", zap.Error(err))
				return
			}
		}

		if newlyOnline {
			msg := &mail.Message{
				Sender:  "admin@steamfriendfinder.appspotmail.com",
				Subject: fmt.Sprintf("%s is playing Team Fortress 2", playerName),
			}
			if err := mail.SendToAdmins(c, msg); err != nil {
				logger.Error("Couldn't send email", zap.Error(err))
				return
			}
			logger.Debug("Sending email")
		}

	}
}

const friendsTable = "friends-history"

func GetRecord(steamid int64) (*FriendSummary, error) {

	output, err := dyndb.GetItem(&dynamodb.GetItemInput{
		TableName: aws.String(friendsTable),
		Key: map[string]*dynamodb.AttributeValue{
			"steamid": {
				N: aws.String(strconv.FormatInt(steamid, 10)),
			},
		},
	})
	if err != nil {
		return nil, err
	}

	item := output.Item
	var friend FriendSummary
	if item == nil {
		// always return a valid friend
		friend.Steamid = steamid
		return &friend, nil
	}

	err = dynamodbattribute.UnmarshalMap(item, &friend)
	if err != nil {
		return nil, err
	}

	return &friend, nil
}

func SaveRecord(friend *FriendSummary) error {

	av, err := dynamodbattribute.MarshalMap(friend)
	if err != nil {
		return err
	}

	var expressions = make([]string, 0)
	for key, value := range av {
		delete(av, key)
		if key == "steamid" {
			continue
		}

		av[":"+key] = value
		expressions = append(expressions, fmt.Sprintf("%s = :%s", key, key))
	}
	updateExpression := "SET " + strings.Join(expression, ", ")

	_, err := dyndb.UpdateItem(&dynamodb.UpdateItemInput{
		TableName: aws.String(friendsTable),
		Key: map[string]*dynamodb.AttributeValue{
			"steamid": {
				N: aws.String(strconv.FormatInt(steamid < 10)),
			},
		},
		UpdateExpression:          &updateExpression,
		ExpressionAttributeValues: av,
	})

	if err != nil {
		return err
	}
}

func main() {
	http.HandleFunc("/", handler)
}
