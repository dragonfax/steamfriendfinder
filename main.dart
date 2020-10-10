import 'package:aws_lambda_dart_runtime/aws_lambda_dart_runtime.dart';
import "package:aws_lambda_dart_runtime/runtime/context.dart";
import 'package:aws_secretsmanager_api/secretsmanager-2017-10-17.dart';
import "package:aws_sqs_api/sqs-2012-11-05.dart";
import "package:aws_sns_api/sns-2010-03-31.dart";
import "package:aws_dynamodb_api/dynamodb-2012-08-10.dart";
import "enums.dart";
import "friend.dart";

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

Future<InvocationResult> steamCron(Context context, AwsCloudwatchEvent event) async {
  return InvocationResult( context.requestId, "OK");
}

Future<InvocationResult> steamSQS(context, AwsSQSEvent event) async {
  return InvocationResult( context.requestId, "OK");
}

const awsRegion = "us-west-2";

var dynamodb;
var sns;
var sqs;
var ssm;

var steamToken;
var phoneNumber;

String queueURL;

void main() async {

  var awsCreds = AwsClientCredentials( );
  dynamodb = DynamoDB(region: awsRegion, credentials: awsCreds);
  sns = SNS(region: awsRegion, credentials: awsCreds);
  sqs = SQS(region: awsRegion, credentials: awsCreds);
  ssm = SecretsManager(region:awsRegion, credentials: awsCreds);

  steamToken = (await ssm.getSecretValue(secretId: "steam_token")).secretString;
  phoneNumber = (await ssm.getSecretValue(secretId: "phone_number")).secretString;

  /// The Runtime is a singleton.
  /// You can define the handlers as you wish.
  Runtime()
    ..registerHandler<AwsCloudwatchEvent>("steam.Cron", steamCron)
    ..registerHandler<AwsSQSEvent>("steam.SQS", steamSQS)
    ..invoke();
}

