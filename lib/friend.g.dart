// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'friend.dart';

// **************************************************************************
// JsonSerializableGenerator
// **************************************************************************

Friend _$FriendFromJson(Map<String, dynamic> json) {
  return Friend()
    ..steamID = json['steamid'] as String
    ..communityVisibleState = json['communityvisibilitystate'] as int
    ..personaName = json['personaname'] as String
    ..lastLogOff = json['lastlogoff'] as int
    ..profileURL = json['profileurl'] as String
    ..personaState = json['personastate'] as int
    ..realName = json['realname'] as String
    ..gameExtraInfo = json['gameextrainfo'] as String
    ..gameID = json['gameid'] as String;
}

Map<String, dynamic> _$FriendToJson(Friend instance) => <String, dynamic>{
      'steamid': instance.steamID,
      'communityvisibilitystate': instance.communityVisibleState,
      'personaname': instance.personaName,
      'lastlogoff': instance.lastLogOff,
      'profileurl': instance.profileURL,
      'personastate': instance.personaState,
      'realname': instance.realName,
      'gameextrainfo': instance.gameExtraInfo,
      'gameid': instance.gameID,
    };
