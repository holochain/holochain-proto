// Get list of chat Spaces / Rooms / Channels
expose("listRooms", HC.JSON);
function listRooms() {
  var rooms = getlink(App.DNA.Hash, "room");
  if( rooms instanceof Error ){
      return []
  } else {
    rooms = rooms.Entries
    var return_rooms = new Array(rooms.length);
    for( i=0; i<rooms.length; i++) {
      return_rooms[i] = JSON.parse(rooms[i]["E"]["C"])
      return_rooms[i].id = rooms[i]["H"]
    }
    return return_rooms
  }
}

// Create a new chat Space / Channel
expose("newRoom", HC.JSON);
function newRoom(x) {
  var key = commit("room", x);
  put(key)
  putmeta(App.DNA.Hash, key, "room")
  return key
}

function isAllowed(author) {
    debug("Checking if "+author+" is a registered user...");
    debug(JSON.stringify(App));

    var registered_users = getlink(App.DNA.Hash, "registered_users");
    debug("Registered users are: "+JSON.stringify(registered_users));
    if( registered_users instanceof Error ) return false;
    registered_users = registered_users.Entries;
    for(var i=0; i < registered_users.length; i++) {
        var profile = JSON.parse(registered_users[i]["E"]["C"]);
        debug("Registered user "+i+" is " + profile.username);
        if( profile.agent_id == author) return true;
    }
    return false;
}

function genesis() {
  return true;
}

function validatePut(entry_type,entry,header,sources) {
    return validate(entry_type,entry,header,sources);
}
function validateCommit(entry_type,entry,header,sources) {
    return validate(entry_type,entry,header,sources);
}
// Local validate an entry before committing ???
function validate(entry_type,entry,header,sources) {
    return isAllowed(sources[0]);
}

function validateLink(linkingEntryType,baseHash,linkHash,tag,sources){return true}
