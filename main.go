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

var games = []string{
	"440",
	"945360",
	"275850",
	"1097150",
	"1062830",
	"477160",
	"246900",
	"312670",
	"1057240",
	"552500",
	"526870",
}

const friendsTable = "friends-history"
const (
	// personastate
	OFFLINE         = 0
	ONLINE          = 1
	BUSY            = 2
	AWAY            = 3
	SNOOZE          = 4
	TRADING         = 5
	LOOKING_TO_PLAY = 6

	// visibility state
	INVISIBLE = 1
	VISIBLE   = 3
)

var awsSess = session.Must(session.NewSession())
var dyndb = dynamodb.New(awsSess)

var logger, _ = zap.NewProduction()

type FriendSummary struct {
	SteamID                  string `json:"steamid"`
	CommunityVisibilityState uint   `json:"communityvisibilitystate"`
	PersonaName              string `json:"personaname"`
	LastLogOff               uint   `json:"lastlogoff"`
	ProfileURL               string `json:"profileurl"`
	PersonaState             uint   `json:"personastate"`
	RealName                 string `json:"realname"`
	GameExtraInfo            string `json:"gameextrainfo"`
	GameID                   string `json:"gameid"`
}

type PlayerSummariesResult struct {
	Response struct {
		Players []*FriendSummary
	}
}

func fetchPlayerSummaries(steamIDs []string) (*PlayerSummariesResult, error) {

	url := fmt.Sprintf("http://api.steampowered.com/ISteamUser/GetPlayerSummaries/v0002/?key=%s&steamids=%s", TOKEN, strings.Join(steamIDs, ","))
	logger.Debug("steam API url", zap.String("url", url))
	resp, err := http.Get(url)
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

func queryPlayerHistories() ([]*FriendSummary, error) {

	output, err := dyndb.Scan(&dynamodb.ScanInput{
		TableName: aws.String(friendsTable),
	})
	if err != nil {
		return nil, err
	}

	var frienstList = make([]*FriendSummary, 0)
	err = dynamodbattribute.UnmarshalListOfMaps(output.Items, &frienstList)
	if err != nil {
		return nil, err
	}

	return frienstList, nil
}

func stringInList(list []string, str string) bool {
	for _, s2 := range list {
		if str == s2 {
			return true
		}
	}
	return false
}

func handleCronEvent() error {

	histories, err := queryPlayerHistories()
	if err != nil {
		return err
	}

	steamIDs := make([]string, 0)
	steamIDtoHistory := make(map[string]*FriendSummary)
	for _, history := range histories {
		steamIDs = append(steamIDs, history.SteamID)
		steamIDtoHistory[history.SteamID] = history
	}

	summaries, err := fetchPlayerSummaries(steamIDs)
	if err != nil {
		logger.Error("failed to retrieve player status: %v", zap.Error(err))
		return err
	}

	for _, summary := range summaries.Response.Players {

		// no longer visible to us.
		if !(summary.CommunityVisibilityState == VISIBLE && summary.PersonaState == ONLINE) {
			continue
		}

		// not in a game.
		if summary.GameID == 0 {
			continue
		}

		history := steamIDtoHistory[summary.SteamID]
		if history == nil {
			panic("no history for followed friend")
		}

		if history.GameID == summary.GameID {
			// same game since last itme.
			continue
		}

		if !stringInList(games, summary.GameID) {
			// not one of our games.
			continue
		}

		notify(summary.PersonaName, summary.GameExtraInfo)

	}

	// save records
}

/*
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
	if item == nil {
		return nil, errors.New("missing record for friend.")
	}

	var friend FriendSummary
	err = dynamodbattribute.UnmarshalMap(item, &friend)
	if err != nil {
		return nil, err
	}

	return &friend, nil
}
*/

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
