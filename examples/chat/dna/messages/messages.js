// Get list of posts in a Space
function listMessages(room) {
    var messages = getLink(room, "message",{Load:true});
  if( messages instanceof Error ) {
    return []
  } else {
    messages = messages.Links
    var return_messages = new Array(messages.length);
    for( i=0; i<messages.length; i++) {
      return_messages[i] = JSON.parse(messages[i]["E"])
      return_messages[i].id = messages[i]["H"]
    }
    return return_messages
  }
}
// TODO Replace edited posts. Drop deleted/invalidated ones.


// Create a new post in a Space / Channel
// receives content, room, [inReplyTo]
function newMessage(x) {
    x.timestamp = new Date();
    var key = commit("message", x);
    commit("my_messages",{Links:[{Base:x.room,Link:key,Tag:"message"}]})
    return key
}


// Edit a post (create new one which "replaces" the old)
// receives message like in newMessage and old_message's hash
function modMessage(x, old_message) {
    var key = commit("message", x);
    commit("my_messages",{Links:[{Base:old_post,Link:key,Tag:"replacedBy"}]})
    return key
}

function isAllowed(author) {
    debug("Checking if "+author+" is a registered user...")
    var registered_users = getLink(App.DNA.Hash, "registered_users",{Load:true});
    debug("Registered users are: "+JSON.stringify(registered_users));
    if( registered_users instanceof Error ) return false;
    registered_users = registered_users.Links
    for(var i=0; i < registered_users.length; i++) {
        var profile = JSON.parse(registered_users[i]["E"])
        debug("Registered user "+i+" is " + profile.username)
        if( profile.agent_id == author) return true;
    }
    return false;
}

function isValidRoom(room) {
    debug("Checking if "+room+" is a valid...")
    var rooms = getLink(App.DNA.Hash, "room",{Load:true});
    debug("Rooms: " + JSON.stringify(rooms))
  if( rooms instanceof Error ){
      return false
  } else {
    rooms = rooms.Links
    for( i=0; i<rooms.length; i++) {
      if( rooms[i]["H"] == room) return true
    }
    return false
  }
}

function genesis() {
  return true;
}


function validatePut(entry_type,entry,header,pkg,sources) {
    return validate(entry_type,entry,header,sources);
}
function validateCommit(entry_type,entry,header,pkg,sources) {
    if (entry_type == "my_messages") {
        //TODO proper source validation here...
        return true;
    }
    return validate(entry_type,entry,header,sources);
}
// Local validate an entry before committing ???
function validate(entry_type,entry,header,sources) {
    if( !isValidRoom(entry.room) ) {
        debug("message not valid because room "+entry.room+" does not exist");
        return false;
    }
    if( isAllowed(sources[0]) ) {
        debug("message \""+entry.content+"\" valid and added to room "+entry.room);
        return true;
    } else {
        return false;
    }
}

function validateLink(linkingEntryType,baseHash,linkHash,tag,pkg,sources){
    return true;
}
function validateMod(entry_type,hash,newHash,pkg,sources) {return true;}
function validateDel(entry_type,hash,pkg,sources) {return true;}
function validatePutPkg(entry_type) {return null}
function validateModPkg(entry_type) { return null}
function validateDelPkg(entry_type) { return null}
function validateLinkPkg(entry_type) { return null}
