// Get list of chat Users / Members
expose("listMembers", HC.JSON);
function listMembers() {return getmeta(property("_id"), "member");}

// Get list of chat Admins
expose("listAdmins", HC.JSON);
function listAdmins() {return getmeta(property("_id"), "admin");}

// Authorize a new agent_id to participate in this holochain
// agent_id must match the string they use to "hc init" their holochain, and is currently their email by convention
expose("addMember", HC.STRING);
function addMember(x) {
  putmeta(property("_id"), x, "member")
}

// Initialize by adding agent to holochain
function genesis() {
  //putmeta(property("_id"), property("_agent_id"), "member");
  //putmeta(property("_id"), property("_agent_id"), "room");
  return true;
}

// Local validate an entry before committing ???
function validate(entry_type, entry, validation_props) {
  return true;
  if( validation_props.MetaTag ) { //validating a putmeta
    return true;
  } else { //validating a commit
    return false
  }
}


//
