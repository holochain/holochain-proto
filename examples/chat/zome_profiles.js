expose("register", HC.JSON);
function register(x) {
  x.agent_id = property("_agent_id")
  var key = commit("profile", x);
  put(key)
  putmeta(property("_id"), key, "registered_users")
  return key
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

function genesis() {
  return true;
}

// Local validate an entry before committing ???
function validate(entry_type, entry, validation_props) {
  if( validation_props.MetaTag ) { //validating a putmeta
    return true;
  } else { //validating a commit or put
    return validation_props.Sources[0] == entry.agent_id
  }
}
