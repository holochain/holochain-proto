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
  var allowed_agents = getmeta(property("_id"), "member");
  for(var i=0; i < allowed_agents.length; i++) {
    if( allowed_agents[i] == author) return true;
  }
  return false;
}

function genesis() {
  return true;
}

// Local validate an entry before committing ???
function validate(entry_type, entry, validation_props) {
  return true;
  if( validation_props.MetaTag ) { //validating a putmeta
    return true;
  } else { //validating a commit
    return true;
    isAllowed(validation_props.Sources[0])
  }
}
