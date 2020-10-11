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

  @JsonKey(name: "steamid")
  String steamID;

  @JsonKey(name: "communityvisibilitystate")
  int communityVisibleState;

  @JsonKey(name: "personaname")
  String personaName;

  @JsonKey(name: "lastlogoff")
  int lastLogOff;

  @JsonKey(name: "profileurl")
  String profileURL;

  @JsonKey(name: "personastate")
  int personaState;

  @JsonKey(name: "realname")
  String realName;

  @JsonKey(name: "gameextrainfo")
  String gameExtraInfo;

  @JsonKey(name: "gameid")
  String gameID;

  factory Friend.fromJson(Map<String, dynamic> json) => _$FriendFromJson(json);

  Map<String, dynamic> toJson() => _$FriendToJson(this);

  factory Friend.fromDynamoDB(Map<String, AttributeValue> dyn) {
    var friend = Friend();
    friend.steamID = dyn["steamid"].s;
    friend.communityVisibleState = int.parse(dyn["communityvisibilitystate"].n);
    friend.personaName = dyn["personaname"].s;
    friend.lastLogOff = int.parse(dyn["lastlogoff"].n);
    friend.profileURL = dyn["profileurl"].s;
    friend.personaState = int.parse(dyn["personastate"].n);
    friend.realName = dyn["realname"].s;
    friend.gameExtraInfo = dyn["gameextrainfo"].s;
    friend.gameID = dyn["gameid"].s;
    return friend;
  }

  UpdateReturn toDynamoDBUpdate() {
    var attributes = {
      ":communityvisibilitystate": this.communityVisibleState,
      ":personaname": this.personaName,
      ":lastlogoff": this.lastLogOff,
      ":profileurl": this.profileURL,
      ":personastate": this.personaState,
      ":realname": this.realName,
      ":gameextrainfo": this.gameExtraInfo,
      ":gameid": this.gameID
    };
    var updateExpression = "SET communityvisibilitystate = :communityvisibilitystate, personaname = :personaname, lastlogoff = :lastlogoff, profileurl = :profileurl, personastate = :personastate, realname = :realname, gameextrainfo = :gameextrainfo, gameid = :gameid";
    return UpdateReturn(updateExpression, attributes);
  }

  save(String table, DynamoDB dynamodb) {
    var key = { "steamid": AttributeValue(s: this.steamID) };
    var r = toDynamoDBUpdate();
    dynamodb.updateItem(key: key, tableName: table, updateExpression: r.updateExpression, expressionAttributeValues: r.attributes);
  }

}

