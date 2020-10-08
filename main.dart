import 'package:aws_lambda_dart_runtime/aws_lambda_dart_runtime.dart';

void main() async {
    final Handler<AwsCloudwatchEvent> steamCron = (context, event) async {

    return InvocationResult(
        context.requestId, AwsALBResponse.fromString(response));
  };
  /// The Runtime is a singleton.
  /// You can define the handlers as you wish.
  Runtime()
    ..registerHandler<AwsCloudwatchEvent>("steam.Cron", steamCron)
    ..invoke();
}

