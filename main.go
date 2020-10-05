package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/aws/aws-sdk-go/service/secretsmanager"
	"github.com/aws/aws-sdk-go/service/sns"
	"github.com/aws/aws-sdk-go/service/sqs"
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

aws lambda cron event
{
	"version":"0",
	"id":"b7a98e0c-aba9-7cfd-fa82-932bc343eb95",
	"detail-type":"Scheduled Event",
	"source":"aws.events",
	"account":"623157150824",
	"time":"2020-10-05T03:50:53Z",
	"region":"us-west-2",
	"resources":[
		"arn:aws:events:us-west-2:623157150824:rule/steamfinder-dev-SteamfinderEventsRuleSchedule1-H1C1DNQNW6UY"
	],
	"detail":{}
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
	"1222730", // star wars squadons
}

const queueName = "FriendQueue"

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
var sqsSess = sqs.New(awsSess)
var snsSess = sns.New(awsSess)
var ssmSess = secretsmanager.New(awsSess)

var steamToken string
var phoneNumber string

func init() {
	output, err := ssmSess.GetSecretValue(&secretsmanager.GetSecretValueInput{
		SecretId: aws.String("steam_token"),
	})
	if err != nil {
		panic(err)
	}
	steamToken = *output.SecretString

	output, err = ssmSess.GetSecretValue(&secretsmanager.GetSecretValueInput{
		SecretId: aws.String("phone_number"),
	})
	if err != nil {
		panic(err)
	}
	phoneNumber = *output.SecretString
}

var queueURL *string

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

func fetchPlayerSummaries(steamIDs []string) ([]*FriendSummary, error) {

	url := fmt.Sprintf("http://api.steampowered.com/ISteamUser/GetPlayerSummaries/v0002/?key=%s&steamids=%s", steamToken, strings.Join(steamIDs, ","))
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

	return response.Response.Players, nil
}

type Event struct {
	Type string `json:"detail-type"`
}

func handler(eventJS json.RawMessage) error {

	var event Event
	err := json.Unmarshal(eventJS, &event)
	if err != nil {
		logger.Error("error while unmarshaling the incoming event", zap.Error(err), zap.String("event", string(eventJS)))
		return nil
	}

	if event.Type == "Scheduled Event" {
		err = handleCronEvent()
		if err != nil {
			logger.Error("error in cron handler", zap.Error(err), zap.Error(err), zap.String("event", string(eventJS)))
		}
	} else {
		logger.Info("incoming sqs event", zap.ByteString("event", eventJS))
		// is SQS event
		var events events.SQSEvent
		err := json.Unmarshal(eventJS, &events)
		if err != nil {
			logger.Error("error while unmarshaling the incoming SQSEvent", zap.Error(err), zap.String("event", string(eventJS)))
			return nil
		}
		handleSQSEvents(events)
	}

	return nil
}

func handleSQSEvents(sqsEvent events.SQSEvent) {

	for _, message := range sqsEvent.Records {
		err := handleSQSEvent(message)
		if err != nil {
			logger.Error("error occured during SQL event", zap.Error(err), zap.Any("message", message))
			// we want to skip errored events, not retry them
		}
	}
}

func handleSQSEvent(message events.SQSMessage) error {

	// get the steam id from the event.
	steamID := *message.MessageAttributes["SteamID"].StringValue
	name := *message.MessageAttributes["Name"].StringValue
	game := *message.MessageAttributes["Game"].StringValue
	gameID := *message.MessageAttributes["GameID"].StringValue

	summaries, err := fetchPlayerSummaries([]string{steamID})
	if err != nil {
		return err
	}

	if len(summaries) != 1 {
		return errors.New("no response from fetching a player")
	}

	if gameID == summaries[0].GameID {
		err = notify(name, game)
		if err != nil {
			return err
		}
	}

	return nil
}

func notify(name, game string) error {
	_, err := snsSess.Publish(&sns.PublishInput{
		PhoneNumber: aws.String(phoneNumber), // +1XXXXXXXXXX
		Message:     aws.String(fmt.Sprintf("%s is playing %s", name, game)),
		MessageAttributes: map[string]*sns.MessageAttributeValue{
			"AWS.SNS.SMS.SMSType": {
				StringValue: aws.String("Transactional"),
			},
			"AWS.SNS.SMS.MaxPrice": {
				StringValue: aws.String("0.01"),
			},
			"AWS.SNS.SMS.SenderID": {
				StringValue: aws.String("stmfriends"),
			},
		},
	})
	return err
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

	for _, summary := range summaries {

		// no longer visible to us.
		if !(summary.CommunityVisibilityState == VISIBLE && summary.PersonaState == ONLINE) {
			continue
		}

		// not in a game.
		if summary.GameID == "" {
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

		err := queue(summary)
		if err != nil {
			// logger.Info("error occured while queiing", zap.Error(err))
			return err
		}

	}

	// save records
	err = SaveFriends(summaries)
	if err != nil {
		return err
	}

	return nil
}

// const queueMessageDelay = 60 * 60 * 10 // 10 minutes
const queueMessageDelay = 2 // dev TODO

func queue(friend *FriendSummary) error {
	logger.Info("queuing player", zap.Any("friend", friend))
	_, err := sqsSess.SendMessage(&sqs.SendMessageInput{
		DelaySeconds: aws.Int64(queueMessageDelay),
		QueueUrl:     queueURL,
		MessageBody:  aws.String("nothing"),
		MessageAttributes: map[string]*sqs.MessageAttributeValue{
			"SteamID": &sqs.MessageAttributeValue{
				DataType:    aws.String("String"),
				StringValue: aws.String(friend.SteamID),
			},
			"Name": &sqs.MessageAttributeValue{
				DataType:    aws.String("String"),
				StringValue: aws.String(friend.PersonaName),
			},
			"GameID": &sqs.MessageAttributeValue{
				DataType:    aws.String("String"),
				StringValue: aws.String(friend.GameID),
			},
			"Game": &sqs.MessageAttributeValue{
				DataType:    aws.String("String"),
				StringValue: aws.String(friend.GameExtraInfo),
			},
		},
	})
	return err
}

func SaveFriends(friends []*FriendSummary) error {
	for _, friend := range friends {
		err := SaveFriend(friend)
		if err != nil {
			return err
		}
	}

	return nil
}

func SaveFriend(friend *FriendSummary) error {

	av, err := dynamodbattribute.MarshalMap(friend)
	if err != nil {
		return err
	}

	var expressions = make([]string, 0)
	av2 := make(map[string]*dynamodb.AttributeValue)
	for key, value := range av {
		if key == "steamid" {
			continue
		}

		av2[":"+key] = value
		expressions = append(expressions, fmt.Sprintf("%s = :%s", key, key))
	}
	updateExpression := "SET " + strings.Join(expressions, ", ")

	_, err = dyndb.UpdateItem(&dynamodb.UpdateItemInput{
		TableName: aws.String(friendsTable),
		Key: map[string]*dynamodb.AttributeValue{
			"steamid": {
				S: aws.String(friend.SteamID),
			},
		},
		UpdateExpression:          &updateExpression,
		ExpressionAttributeValues: av2,
	})

	if err != nil {
		logger.Error("error updating dynamodb", zap.Error(err), zap.String("update expression", updateExpression), zap.Any("attribute values", av2))
		return err
	}

	return nil
}

func main() {

	output, err := sqsSess.GetQueueUrl(&sqs.GetQueueUrlInput{
		QueueName: aws.String(queueName),
	})
	if err != nil {
		panic(err)
	}
	queueURL = output.QueueUrl

	lambda.Start(handler)
}
