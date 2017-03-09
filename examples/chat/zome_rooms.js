// Get list of chat Spaces / Rooms / Channels
expose("listRooms", HC.JSON);
function listRooms() {
  var rooms = getmeta(property("_id"), "room");
  if( rooms instanceof Error ){
      return []
  } else {
    var return_rooms = new Array(rooms.length);
    for( i=0; i<rooms.length; i++) {
      return_rooms[i] = JSON.parse(rooms[i]["C"])
    }
    return return_rooms
  }
}

// Create a new chat Space / Channel
expose("newRoom", HC.JSON);
function newRoom(x) {
  var key = commit("room", x);
  put(key)
  putmeta(property("_id"), key, "room")
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
function validate(entry_type, entry, validation_props) {;
  if( validation_props.MetaTag ) { //validating a putmeta
    return true;
  } else { //validating a commit
    return isAllowed(validation_props.Sources[0])
  }
}
