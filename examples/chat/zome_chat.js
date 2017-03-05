/* Query a property of Holochain DNA / Config
Known properties to retrieve:
 * ID (hash of holochain DNA which uniquely identifies this holochain)
 * Name (name of team/group/holochain)
 * Purpose (description of team/holochain)
 * Agent_ID (agent string used to initialize this holochain (normally email address by convention))
*/
// expose("getProperty");
// function getProperty(x) {return property(x);}

// Get list of chat Spaces / Rooms / Channels
expose("listRooms", HC.JSON);
function listRooms() {return getmeta(property("_id"), "room");}

// Get list of chat Users / Members
expose("listMembers", HC.JSON);
function listMembers() {return getmeta(property("_id"), "member");}

// Get list of chat Admins
expose("listAdmins", HC.JSON);
function listAdmins() {return getmeta(property("_id"), "admin");}

// Get list of posts in a Space
expose("listMessages", HC.JSON);
function listMessages(room) {
  var message_keys = getmeta(room, "message");
  var messages = new Array(message_keys.length);
  for( i=0; i<message_keys.length; i++) {
    messages[i] = get(message_keys[i]);
  }
  return messages;
}
// TODO Replace edited posts. Drop deleted/invalidated ones.

// Authorize a new agent_id to participate in this holochain
// agent_id must match the string they use to "hc init" their holochain, and is currently their email by convention
expose("addMember", HC.STRING);
function addMember(x) {
  putmeta(property("_id"), x, "member")
}

// Create a new chat Space / Channel
expose("newRoom", HC.JSON);
function newRoom(x) {
  var key = commit("room", x);
  put(key)
  putmeta(property("_id"), key, "room")
  return key
}

// Create a new post in a Space / Channel
expose("newMessage", HC.JSON); // receives content, room, [inReplyTo]
function newMessage(x) {
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

expose("newProfile", HC.JSON);
function newProfile(x) {
  x.agent_id = property("_agent_id")
  return commit("profile", x);
}

// Get profile information for a user
expose("getProfile", HC.JSON); // receives a user hashkey
function getProfile(x) {
  return get(x);
}

// Update profile information for an agent_id
expose("modProfile", HC.JSON);
function modProfile(x, old_profile) {
  var key = commit("profile", x);
  put(key)
  putmeta(old_profile, key, "replacedBy")
}

// Initialize by adding agent to holochain
function init() {
  putmeta(property("_id"), property("_agent_id"), "member")};
}

function isAllowed(author) {
  var allowed_agents = getmeta(property("_id"), "member");
  for(var i=0; i < allowed_agents.length; i++) {
    if( allowed_agents[i] == author) return true;
  }
  return false;
}

// Local validate an entry before committing ???
function validate(entry_type, entry, validation_props) {
  if( validation_props.MetaTag ) { //validating a putmeta

  } else { //validating a commit
    if( entry_type == "message" || entry_type == "room" ) {
      return isAllowed(validation_props.Sources[0])
    }

    if( entry_type == "profile" ) {
      return validation_props.Sources[0] == entry.agent_id
    }
  }
}


//
