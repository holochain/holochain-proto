function register(x) {
    x.agent_id = App.Key.Hash
    var key = commit("profile", x);
    commit("registrations",{Links:[{Base:App.DNA.Hash,Link:key,Tag:"registered_users"}]})
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

function validatePut(entry_type,entry,header,pkg,sources) {
    return validate(entry_type,entry,header,sources);
}
function validateCommit(entry_type,entry,header,pkg,sources) {
    if (entry_type == "registrations") {
        debug("registration entry:"+JSON.stringify(entry));
        var links = entry.Links;
        for(var i=0; i < links.length; i++) {
            var l = links[i]
            debug("link: "+JSON.stringify(l))

            // registrations all must happen on the DNA & only a source can register itself
            if (l.Base != App.DNA.Hash) {
                debug("validation failed, expected reg base to be: "+App.DNA.Hash+" but was: "+l.Base)
                return false;
            }
        }
        return true;
    }
    return validate(entry_type,entry,header,sources);
}
// Local validate an entry before committing ???
function validate(entry_type,entry,header,sources) {
    return sources[0] == entry.agent_id;
}

function validateLink(linkingEntryType,baseHash,linkHash,tag,pkg,sources){
    return true;
}
function validateMod(entry_type,hash,newHash,pkg,sources) {return true;}
function validateDel(entry_type,hash,pkg,sources) {return true;}
function validatePutPkg(entry_type) {return null}
function validateModPkg(entry_type) { return null}
function validateDelPkg(entry_type) { return null}
function validateLinkPkg(entry_type) { return null}
