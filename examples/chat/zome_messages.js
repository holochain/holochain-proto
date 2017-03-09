// Get list of posts in a Space
expose("listMessages", HC.JSON);
function listMessages(room) {
  var messages = getmeta(room, "message");
  if( messages instanceof Error ) {
    return []
  } else {
    var return_messages = new Array(messages.length);
    for( i=0; i<messages.length; i++) {
      return_messages[i] = JSON.parse(messages[i]["C"])
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
  var registered_users = getmeta(property("_id"), "registered_users");
  for(var i=0; i < registered_users.length; i++) {
    var profile = JSON.parse(registered_users[i]["C"])
    debug("Registered user "+i+" is " + profile.username)
    if( profile.agent_id == author) return true;
  }
  return false;
}

function genesis() {
  return true;
}

// Local validate an entry before committing ???
function validate(entry_type, entry, validation_props) {
  if( validation_props.MetaTag ) { //validating a putmeta
    return true;
  } else { //validating a commit or put
    return isAllowed(validation_props.Sources[0])
  }
}
