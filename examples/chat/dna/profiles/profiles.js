function register(x) {
    x.agent_id = App.Key.Hash
    var key = commit("profile", x);
    commit("registrations", {Links:[{Base:App.DNA.Hash,Link:key,Tag:"registered_users"}]})
    commit("agent_profile_link", { Links:[{
      Base: App.Key.Hash,
      Link: key,
      Tag: "profile"
    }]})
    return key
}

function isRegistered() {
    var registered_users = getLink(App.DNA.Hash, "registered_users",{Load:true})
    debug("Registered users are: "+JSON.stringify(registered_users));
    if( registered_users instanceof Error) return false
    registered_users = registered_users.Links
    var agent_id = App.Key.Hash
    for(var i=0; i < registered_users.length; i++) {
        var profile = JSON.parse(registered_users[i]["E"])
        debug("Registered user "+i+" is " + profile.username)
        if( profile.agent_id == agent_id) return true;
    }
    return false;
}

// Get profile information for a user
// receives a user hashkey
function getProfile(x) {
    return get(x);
}

function myProfile() {
    var registered_users = getLink(App.DNA.Hash, "registered_users",{Load:true});
    if( registered_users instanceof Error ) return false
    debug("registration entry:"+JSON.stringify(registered_users))
    registered_users = registered_users.Links
    var agent_id = App.Key.Hash
    for(var i=0; i < registered_users.length; i++) {
        var profile = JSON.parse(registered_users[i]["E"])
        debug("Registered user "+i+" is " + profile.username)
        if( profile.agent_id == agent_id) return profile;
    }
    return false;
}

// Update profile information for an agent_id
function modProfile(x, old_profile) {
    var key = commit("profile", x);
    commit("registrations",{Links:[{Base:old_profile,Link:key,Tag:"replacedBy"}]})
    return key
}

function genesis() {
    return true;
}

function isSourcesOwnProfile(entry, sources) {
    return sources[0] == entry.agent_id;
}

function isRegistrationOnDNA(registration_entry) {
  debug("registration entry:"+JSON.stringify(registration_entry));
  var links = registration_entry.Links;
  for(var i=0; i < links.length; i++) {
      var l = links[i]
      debug("link: "+JSON.stringify(l))

      if (l.Base != App.DNA.Hash) {
          debug("validation failed, expected reg base to be: "+App.DNA.Hash+" but was: "+l.Base)
          return false;
      }
  }
  return true;
}

function isLinkFromSource(entry, sources) {
  if(entry.Links.length != 1) {
    debug("validation failed, expected agent_profile_link to contain exactly one link")
    return false
  }

  if(entry.Links[0].Base != sources[0]) {
    debug("validation failed, expected agent_profile_link to link from the source")
    return false
  }

  return true
}

function validatePut(entry_type,entry,header,pkg,sources) {
    return validateCommit(entry_type,entry,header,pkg,sources)
}
function validateCommit(entry_type,entry,header,pkg,sources) {
    // registrations all must happen on the DNA
    if (entry_type == "registrations") {
        return isRegistrationOnDNA(entry)
    }

    // can only link from my profile
    if (entry_type == "agent_profile_link" ){
        return isLinkFromSource(entry, sources)
    }

    // nobody can add somebody elses profile
    return isSourcesOwnProfile(entry, sources);
}



function validateLink(linkingEntryType,baseHash,linkHash,pkg,sources){
    // can only link from my profile
    if (linkingEntryType == "agent_profile_link" ){
        return baseHash == sources[0];
    }

    // registrations all must happen on the DNA
    if (linkingEntryType == "registrations") {
        return baseHash == App.DNA.Hash
    }

    return true
}
function validateMod(entry_type,hash,newHash,pkg,sources) {return true;}
function validateDel(entry_type,hash,pkg,sources) {return true;}
function validatePutPkg(entry_type) {return null}
function validateModPkg(entry_type) { return null}
function validateDelPkg(entry_type) { return null}
function validateLinkPkg(entry_type) { return null}
