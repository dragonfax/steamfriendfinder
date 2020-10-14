import 'package:aws_lambda_dart_runtime/aws_lambda_dart_runtime.dart';
import "package:aws_lambda_dart_runtime/runtime/context.dart";
import 'package:aws_secretsmanager_api/secretsmanager-2017-10-17.dart';
import "package:aws_sqs_api/sqs-2012-11-05.dart";
import "package:aws_sns_api/sns-2010-03-31.dart" as snslib;
import "package:aws_dynamodb_api/dynamodb-2012-08-10.dart";

import "enums.dart";
import "lib/friend.dart";
import "dart:io";
import "dart:convert";

var games = <String>[
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
];

const queueName = "FriendQueue";
const friendsTable = "friends-history";

const maxSMSPrice = "0.05";

const queueMessageDelay = 60 * 10;  // 10 minutes

const awsRegion = "us-west-2";

DynamoDB dynamodb;
snslib.SNS sns;
SQS sqs;
SecretsManager ssm;

String steamToken;
String phoneNumber;

String queueURL;

Future<List<Friend>> fetchPlayerSummaries(List<String> steamIDs) async {

  final url = "http://api.steampowered.com/ISteamUser/GetPlayerSummaries/v0002/?key=$steamToken&steamids=${steamIDs.join(',')}";
  final request = await HttpClient().getUrl(Uri.parse(url));
  final response = await request.close(); 
  if ( response.statusCode != 200 ) {
    throw "wrong status code ${response.statusCode}";
  }
  final js = await utf8.decodeStream(response);
  // convert js to friends
  final map = json.decode(js);
  return map['response']['players'];
}

Future<List<Friend>> queryPlayerHistories() async {
  var items = ( await dynamodb.scan(tableName: friendsTable)).items;
  var friends = <Friend>[];
  for ( final item in items ) {
    friends.add(Friend.fromDynamoDB(item));
  }
  return friends;
}

handleCron() async {

  var histories = await queryPlayerHistories();

  var summaries = await fetchPlayerSummaries(histories.map((history) => history.steamID));

  for ( final summary in summaries ) {
    if ( summary.communityVisibleState != Visibility.VISIBLE || summary.personaState != PersonaState.ONLINE ) {
      continue;
    }

    if ( summary.gameID == "" ) {
      continue;
    }

    var history = histories.firstWhere((h) => h.steamID == summary.steamID);
    if ( history == null ) {
      throw("no history for followed friend");
    }

    if ( history.gameID == summary.gameID ) {
      continue;
    }

    if ( ! games.contains(summary.gameID) ) {
      continue;
    }

    queue(summary);
  }

  for ( final friend in summaries ) {
    friend.save(friendsTable, dynamodb);
  }
}

Future<InvocationResult> receiveCron(Context context, AwsCloudwatchEvent event) async {
  await handleCron();
  return InvocationResult( context.requestId, "OK");
}

Future<InvocationResult> receiveSQS(context, AwsSQSEvent event) async {

  for ( var record in event.records ) {
    await handleSQS(record);
  }

  return InvocationResult( context.requestId, "OK");
}

handleSQS(AwsSQSEventRecord record) async {

  var steamID = record.messageAttributes[Friend.steamIDKey];
  var summaries = await fetchPlayerSummaries(<String>[steamID]);
  if ( summaries.length == 0 ) {
    throw "no response for player ${steamID}";
  }
  var summary = summaries[0];

  if ( record.messageAttributes[Friend.gameIDKey] == summary.gameID ) {
    notify(record.messageAttributes[Friend.personaNameKey], record.messageAttributes[Friend.gameExtraInfoKey]);
  }
}

notify(String name, String game) async {
  sns.publish(
    message: "$name is playing $game", 
    phoneNumber: phoneNumber, 
    messageAttributes: {
      "AWS.SNS.SMS.SMSType": snslib.MessageAttributeValue(
        dataType: "String", 
        stringValue: "Transactional"
      ),
      "AWS.SNS.SMS.MaxPrice": snslib.MessageAttributeValue(
        dataType: "String",
        stringValue: maxSMSPrice
      ),
      "AWS.SNS.SMS.SenderID": snslib.MessageAttributeValue(
        dataType: "String",
        stringValue: "stmfriends"
      )
    }
  );
}

queue(Friend friend) async {
  sqs.sendMessage(
    messageBody: "nothing", 
    queueUrl: queueURL, 
    delaySeconds: queueMessageDelay,
    messageAttributes: {
      Friend.steamIDKey: MessageAttributeValue(
        dataType: "String",
        stringValue: friend.steamID
      ),
      Friend.personaNameKey: MessageAttributeValue(
        dataType: "String",
        stringValue: friend.personaName
      ),
      Friend.gameIDKey: MessageAttributeValue(
        dataType: "String", 
        stringValue: friend.gameID
      ),
      Friend.gameExtraInfoKey: MessageAttributeValue(
        dataType: "String",
        stringValue: friend.gameExtraInfo
      )
    }
  );
}

void main() async {

  var accessKey = Platform.environment['AWS_ACCESS_KEY_ID'];
  var secretKey = Platform.environment['AWS_SECRET_ACCESS_KEY'];
  var sessionToken = Platform.environment['AWS_SESSION_TOKEN'];
  var awsCreds = AwsClientCredentials(accessKey: accessKey, secretKey: secretKey, sessionToken: sessionToken);
  dynamodb = DynamoDB(region: awsRegion, credentials: awsCreds);
  sns = snslib.SNS(region: awsRegion, credentials: awsCreds);
  sqs = SQS(region: awsRegion, credentials: awsCreds);
  ssm = SecretsManager(region:awsRegion, credentials: awsCreds);

  steamToken = (await ssm.getSecretValue(secretId: "steam_token")).secretString;
  phoneNumber = (await ssm.getSecretValue(secretId: "phone_number")).secretString;

  queueURL = ( await sqs.getQueueUrl(queueName: queueName)).queueUrl;

  Runtime()
    ..registerHandler<AwsCloudwatchEvent>("steam.Cron", receiveCron)
    ..registerHandler<AwsSQSEvent>("steam.SQS", receiveSQS)
    ..invoke();
}

