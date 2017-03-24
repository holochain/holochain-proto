// Get list of chat Users / Members
expose("listMembers", HC.JSON);
function listMembers() {return getlink(App.DNA.Hash, "member");}

// Get list of chat Admins
expose("listAdmins", HC.JSON);
function listAdmins() {return getlink(App.DNA.Hash, "admin");}

// Authorize a new agent_id to participate in this holochain
// agent_id must match the string they use to "hc init" their holochain, and is currently their email by convention
expose("addMember", HC.STRING);
function addMember(x) {
  putmeta(App.DNA.Hash, x, "member")
}

// Initialize by adding agent to holochain
function genesis() {
  //putmeta(App.DNA.Hash, App.Agent.Hash, "member");
  //putmeta(App.DNA.Hash, App.Agent.Hash, "room");
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
    return false;
}

function validateLink(linkingEntryType,baseHash,linkHash,tag,sources){return true}
