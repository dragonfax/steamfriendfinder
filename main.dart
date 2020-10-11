import 'package:aws_lambda_dart_runtime/aws_lambda_dart_runtime.dart';
import "package:aws_lambda_dart_runtime/runtime/context.dart";
import 'package:aws_secretsmanager_api/secretsmanager-2017-10-17.dart';
import "package:aws_sqs_api/sqs-2012-11-05.dart";
import "package:aws_sns_api/sns-2010-03-31.dart";
import "package:aws_dynamodb_api/dynamodb-2012-08-10.dart";
// import "enums.dart";
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

const maxSMSPrice = "0.05";

const queueMessageDelay = 60 * 60 * 10;  // 10 minutes

const awsRegion = "us-west-2";

DynamoDB dynamodb;
SNS sns;
SQS sqs;
SSM ssm;

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

Future<InvocationResult> receiveCron(Context context, AwsCloudwatchEvent event) async {



  return InvocationResult( context.requestId, "OK");
}

Future<InvocationResult> receiveSQS(context, AwsSQSEvent event) async {

  for ( var record in event.records ) {
    await handleSQS(record);
  }

  return InvocationResult( context.requestId, "OK");
}

handleSQS(AwsSQSEventRecord record) async {

  var steamID = record.messageAttributes["steamid"];
  var summaries = await fetchPlayerSummaries(<String>[steamID]);
  if ( summaries.length == 0 ) {
    throw "no response for player ${steamID}";
  }
  var summary = summaries[0];

  if ( record.messageAttributes["gameID"] == summary.gameID ) {
    notify(record.messageAttributes["name"], record.messageAttributes["game"]);
  }
}

notify(String name, String game) async {
  sns.publish(
    message: "$name is playing $game", 
    phoneNumber: phoneNumber, 
    messageAttributes: {
      "AWS.SNS.SMS.SMSType": MessageAttributeValue(
        dataType: "String", 
        stringValue: "Transactional"
      ),
      "AWS.SNS.SMS.MaxPrice": MessageAttributeValue(
        dataType: "String",
        stringValue: maxSMSPrice
      ),
      "AWS.SNS.SMS.SenderID": MessageAttributeValue(
        dataType: "String",
        stringValue: "stmfriends"
      )
    }
  );
}

void main() async {

  var awsCreds = AwsClientCredentials( );
  dynamodb = DynamoDB(region: awsRegion, credentials: awsCreds);
  sns = SNS(region: awsRegion, credentials: awsCreds);
  sqs = SQS(region: awsRegion, credentials: awsCreds);
  ssm = SecretsManager(region:awsRegion, credentials: awsCreds);

  steamToken = (await ssm.getSecretValue(secretId: "steam_token")).secretString;
  phoneNumber = (await ssm.getSecretValue(secretId: "phone_number")).secretString;

  queueURL = await sqs.getQueueUrl(queueName).queueUrl;

  /// The Runtime is a singleton.
  /// You can define the handlers as you wish.
  Runtime()
    ..registerHandler<AwsCloudwatchEvent>("steam.Cron", receiveCron)
    ..registerHandler<AwsSQSEvent>("steam.SQS", receiveSQS)
    ..invoke();
}

