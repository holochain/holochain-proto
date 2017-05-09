// Get list of chat Spaces / Rooms / Channels
function listRooms() {
    var rooms = getLink(App.DNA.Hash, "room",{Load:true});
    debug("Rooms: " + JSON.stringify(rooms))
    if( rooms instanceof Error ){
        return []
    } else {
        rooms = rooms.Links
        var return_rooms = new Array(rooms.length);
        for( i=0; i<rooms.length; i++) {
            return_rooms[i] = JSON.parse(rooms[i]["E"])
            return_rooms[i].id = rooms[i]["H"]
        }
        return return_rooms
    }
}

// Create a new chat Space / Channel
function newRoom(x) {
    var key = commit("room", x);
    commit("room_links",{Links:[{Base:App.DNA.Hash,Link:key,Tag:"room"}]})
    return key
}

function isAllowed(author) {
    debug("Checking if "+author+" is a registered user...");
    debug(JSON.stringify(App));

    var registered_users = getLink(App.DNA.Hash, "registered_users",{Load:true});
    debug("Registered users are: "+JSON.stringify(registered_users));
    if( registered_users instanceof Error ) return false;
    registered_users = registered_users.Links;
    for(var i=0; i < registered_users.length; i++) {
        var profile = JSON.parse(registered_users[i]["E"]);
        debug("Registered user "+i+" is " + profile.username);
        if( profile.agent_id == author) return true;
    }
    return false;
}

function genesis() {
    return true;
}

function validatePut(entry_type,entry,header,pkg,sources) {
    return validate(entry_type,entry,header,sources);
}
function validateCommit(entry_type,entry,header,pkg,sources) {
    return validate(entry_type,entry,header,sources);
}
// Local validate an entry before committing ???
function validate(entry_type,entry,header,sources) {
    return isAllowed(sources[0]);
}

function validateLink(linkingEntryType,baseHash,linkHash,tag,pkg,sources){return true}
function validateMod(entry_type,hash,newHash,pkg,sources) {return true;}
function validateDel(entry_type,hash,pkg,sources) {return true;}
function validatePutPkg(entry_type) {return null}
function validateModPkg(entry_type) { return null}
function validateDelPkg(entry_type) { return null}
function validateLinkPkg(entry_type) { return null}
