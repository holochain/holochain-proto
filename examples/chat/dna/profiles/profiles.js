expose("register", HC.JSON);
function register(x) {
  x.agent_id = App.Key.Hash
  var key = commit("profile", x);
  put(key)
  putmeta(App.DNA.Hash, key, "registered_users")
  return key
}

expose("isRegistered", HC.JSON);
function isRegistered() {
  var registered_users = getlink(App.DNA.Hash, "registered_users")
  if( registered_users instanceof Error) return false
  registered_users = registered_users.Entries
  var agent_id = App.Key.Hash
  for(var i=0; i < registered_users.length; i++) {
    var profile = JSON.parse(registered_users[i]["E"]["C"])
    debug("Registered user "+i+" is " + profile.username)
    if( profile.agent_id == agent_id) return true;
  }
  return false;
}

// Get profile information for a user
expose("getProfile", HC.JSON); // receives a user hashkey
function getProfile(x) {
  return get(x);
}

expose("myProfile", HC.JSON);
function myProfile() {
  var registered_users = getlink(App.DNA.Hash, "registered_users");
  if( registered_users instanceof Error ) return false
  registered_users = registered_users.Entries
  var agent_id = App.Key.Hash
  for(var i=0; i < registered_users.length; i++) {
    var profile = JSON.parse(registered_users[i]["E"]["C"])
    debug("Registered user "+i+" is " + profile.username)
    if( profile.agent_id == agent_id) return profile;
  }
  return false;
}

// Update profile information for an agent_id
expose("modProfile", HC.JSON);
function modProfile(x, old_profile) {
  var key = commit("profile", x);
  put(key)
  putmeta(old_profile, key, "replacedBy")
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
    return sources[0] == entry.agent_id;
}
