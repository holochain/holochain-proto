function relate(base,link,tag) {
    var rel = {Links:[{Base:base,Link:link,Tag:tag}]};
    return commit("relation",rel);
}

function addPost(x) {
    var id = App.DNA.Hash;
    var nick = App.Agent.String;
    var k = commit("post",x);
    relate(id,k,"_post_by_"+nick);
    return k;
}

function getData(i) {
    debug("getData "+i.what);
    var id = App.DNA.Hash;
    var links;
    if (i.what == "nick") {
        return App.Agent.String;
    }
    else if (i.what == "posts") {
        links = getLink(id,"_post_by_"+i.whom);
    }
    else if (i.what == "follows") {
        links = getLink(id,"_follows_by_"+i.whom);
    }
    else if (i.what == "users") {
        links = getLink(id,"users");
    }
    debug(JSON.stringify(links));
    if( links instanceof Error ){
        return [];
    }
    var i;
    var l = links.Links;
    var len = l.length;
    var result = [];
    for (i = 0; i < len; i++) {
        var val = get(l[i].H);
        result.push(val.C);
    }
    debug("returning: "+JSON.stringify(result));
    return result;
}

function follow(x) {
    var id = App.DNA.Hash;
    var nick = App.Agent.String;
    var k = commit("follow",x);
    relate(id,k,"_follows_by_"+nick);
    return k;
}

// callbacks -----------------------------------------------------
function genesis() {
    var id = App.DNA.Hash;
    debug("id is "+id);
    var k = commit("profile",{nick:App.Agent.String});
    var l = relate(id,k,"users");
/*    if (!err) {
        return true;
    }*/
    return true;
}

function validate(entry_type,entry,meta) {
    debug("validate: "+entry_type);
    if (entry_type=="post") {
        var l = entry.message.length;
        if (l>0 && l<256) {return true;}
        return false;
    }
    if (entry_type=="profile") {
        return true;
    }
    if (entry_type=="follow") {
        return true;
    }
    return false;
}


function validatePut(entry_type,entry,header,sources) {
    return validate(entry_type,entry,header,sources);
}
function validateCommit(entry_type,entry,header,sources) {
    if (entry_type == "relation") {return true}
    return validate(entry_type,entry,header,sources);
}
function validateLink(linkEntryType,baseHash,linkHash,tag,sources){return true}
