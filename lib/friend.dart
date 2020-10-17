import 'package:aws_dynamodb_api/dynamodb-2012-08-10.dart';
import "package:json_annotation/json_annotation.dart";

part 'friend.g.dart';

class UpdateReturn {
  String updateExpression;
  Map<String, AttributeValue> attributes;

  UpdateReturn(this.updateExpression, this.attributes);
}

@JsonSerializable(nullable: false)
class Friend {

  Friend();

  // used to make these JSON keys typesafe
  // as they use several places. (until dynamodb unmarshal generation)
  static const steamIDKey = "steamid"; 
  static const communityVisibleStateKey = "communityvisibilitystate";
  static const personaNameKey = "personaname";
  static const lastLogOffKey = "lastlogoff";
  static const profileURLKey = "profileurl";
  static const personaStateKey = "personastate";
  static const realNameKey = "realname";
  static const gameExtraInfoKey = "gameextrainfo";
  static const gameIDKey = "gameid";

  @JsonKey(name: steamIDKey)
  String steamID;

  @JsonKey(name: communityVisibleStateKey)
  int communityVisibleState;

  @JsonKey(name: personaNameKey)
  String personaName;

  @JsonKey(name: lastLogOffKey)
  int lastLogOff;

  @JsonKey(name: profileURLKey)
  String profileURL;

  @JsonKey(name: personaStateKey)
  int personaState;

  @JsonKey(name: realNameKey)
  String realName;

  @JsonKey(name: gameExtraInfoKey)
  String gameExtraInfo;

  @JsonKey(name: gameIDKey)
  String gameID;

  factory Friend.fromJson(Map<String, dynamic> json) => _$FriendFromJson(json);

  Map<String, dynamic> toJson() => _$FriendToJson(this);

  factory Friend.fromDynamoDB(Map<String, AttributeValue> dyn) {
    var friend = Friend();
    friend.steamID = dyn[steamIDKey]?.s;
    friend.communityVisibleState = int.parse(dyn[communityVisibleStateKey]?.n ?? "0");
    friend.personaName = dyn[personaNameKey]?.s;
    friend.lastLogOff = int.parse(dyn[lastLogOffKey]?.n ?? "0");
    friend.profileURL = dyn[profileURLKey]?.s;
    friend.personaState = int.parse(dyn[personaStateKey]?.n ?? "0");
    friend.realName = dyn[realNameKey]?.s;
    friend.gameExtraInfo = dyn[gameExtraInfoKey]?.s;
    friend.gameID = dyn[gameIDKey]?.s;
    return friend;
  }

  UpdateReturn toDynamoDBUpdate() {
    var attributes = {
      ":" + communityVisibleStateKey: AttributeValue(n: this.communityVisibleState.toString()),
      ":" + personaNameKey: AttributeValue(s: this.personaName),
      ":" + lastLogOffKey: AttributeValue(n: this.lastLogOff.toString()),
      ":" + profileURLKey: AttributeValue(s: this.profileURL),
      ":" + personaStateKey: AttributeValue(n: this.personaState.toString()),
      ":" + realNameKey: AttributeValue(s: this.realName),
      ":" + gameExtraInfoKey: AttributeValue(s: this.gameExtraInfo),
      ":" + gameIDKey: AttributeValue(s: this.gameID),
      ":updated": AttributeValue(s: DateTime.now().toIso8601String())
    };
    var updateExpression = "SET $communityVisibleStateKey = :$communityVisibleStateKey, $personaNameKey = :$personaNameKey, $lastLogOffKey = :$lastLogOffKey, $profileURLKey = :$profileURLKey, $personaStateKey = :$personaStateKey, $realNameKey = :$realNameKey, $gameExtraInfoKey = $gameExtraInfoKey:, $gameIDKey = :$gameIDKey, updated = :updated";
    return UpdateReturn(updateExpression, attributes);
  }


  static Future<Friend> read(String table, DynamoDB dynamodb, String steamid) async {
    var key = { steamIDKey: AttributeValue(s: steamid) };
    var dyn = ( await dynamodb.getItem(key: key, tableName: table)).item;
    return Friend.fromDynamoDB(dyn);
  }

  save(String table, DynamoDB dynamodb) async {
    var key = { steamIDKey: AttributeValue(s: this.steamID) };
    var r = toDynamoDBUpdate();
    await dynamodb.updateItem(
       key: key, 
       tableName: table, 
       updateExpression: r.updateExpression, 
       expressionAttributeValues: r.attributes
     );
  }

}

