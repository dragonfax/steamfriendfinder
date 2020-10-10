import "package:json_annotation/json_annotation.dart";

part 'friend.g.dart';

@JsonSerializable(nullable: false)
class Friend {

  Friend({
    this.steamID, 
    this.communityVisibleState, 
    this.personaName, 
    this.lastLogOff,
    this.profileURL,
    this.personaState,
    this.realName,
    this.gameExtraInfo,
    this.gameID});

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

}

