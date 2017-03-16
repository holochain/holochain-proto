// Get list of posts in a Space
expose("listMessages", HC.JSON);
function listMessages(room) {
  var messages = getmeta(room, "message");
  if( messages instanceof Error ) {
    return []
  } else {
    messages = messages.Entries
    var return_messages = new Array(messages.length);
    for( i=0; i<messages.length; i++) {
      return_messages[i] = JSON.parse(messages[i]["E"]["C"])
      return_messages[i].id = messages[i]["H"]
    }
    return return_messages
  }
}
// TODO Replace edited posts. Drop deleted/invalidated ones.


// Create a new post in a Space / Channel
expose("newMessage", HC.JSON); // receives content, room, [inReplyTo]
function newMessage(x) {
  x.timestamp = new Date();
  var key = commit("message", x);
  put(key)
  putmeta(x.room, key, "message")
  return key
}


// Edit a post (create new one which "replaces" the old)
expose("modMessage", HC.JSON); // receives message like in newMessage and old_message's hash
function modMessage(x, old_message) {
  var key = commit("message", x);
  put(key)
  putmeta(old_post, key, "replacedBy")
  return key
}

function isAllowed(author) {
  debug("Checking if "+author+" is a registered user...")
  var registered_users = getmeta(App.DNAHash, "registered_users");
  if( registered_users instanceof Error ) return false;
  registered_users = registered_users.Entries
  for(var i=0; i < registered_users.length; i++) {
    var profile = JSON.parse(registered_users[i]["E"]["C"])
    //debug("Registered user "+i+" is " + profile.username)
    if( profile.agent_id == author) return true;
  }
  return false;
}

function isValidRoom(room) {
  var rooms = getmeta(App.DNAHash, "room");
  if( rooms instanceof Error ){
      return false
  } else {
    rooms = rooms.Entries
    for( i=0; i<rooms.length; i++) {
      if( rooms[i]["H"] == room) return true
    }
    return false
  }
}

function genesis() {
  return true;
}

// Local validate an entry before committing ???
function validate(entry_type, entry, validation_props) {
  if( validation_props.MetaTag ) { //validating a putmeta
    return true;
  } else { //validating a commit or put
    if( !isValidRoom(entry.room) ) {
      debug("message not valid because room "+entry.room+" does not exist")
      return false
    }
    if( isAllowed(validation_props.Sources[0]) ) {
      debug("message \""+entry.content+"\" valid and added to room "+entry.room)
      return true
    } else {
      return false
    }
  }
}
