// ==============================================================================
// EXPOSED Functions: visible to the UI, can be called via localhost, web browser, or socket
// ===============================================================================


function appProperty(name) {            // The definition of the function you intend to expose
    if (name == "App_Agent_Hash") {return App.Agent.Hash;}
    if (name == "App_Agent_String")  {return App.Agent.String;}
    if (name == "App_Key_Hash")   {return   App.Key.Hash;}
    if (name == "App_DNA_Hash")   {return   App.DNA.Hash;}
    return "Error: No App Property with name: " + name;
}

function newHandle(handle){
    var me = getMe();
    var directory = getDirectory();
    var handles = doGetLink(me,"handle");
    var n = handles.length - 1;
    if (n >= 0) {
        var oldKey = handles[n];
        var key = update("handle",handle,oldKey);

        debug(handle+" is "+key);
        commit("handle_links",
               {Links:[
                   {Base:me,Link:oldKey,Tag:"handle",LinkAction:HC.LinkAction.Del},
                   {Base:me,Link:key,Tag:"handle"}
               ]});
        commit("directory_links",
               {Links:[
                   {Base:directory,Link:oldKey,Tag:"handle",LinkAction:HC.LinkAction.Del},
                   {Base:directory,Link:key,Tag:"handle"}
               ]});
        return key;
    }
    return addHandle(handle);
}

// returns all the handles in the directory
function getHandles() {
    var directory = getDirectory();
    var links = doGetLinkLoad(directory,"handle");

    var handles = [];
    for (var i=0;i <links.length;i++) {
        var h = links[i].handle;
        handles.push({handle:h,agent:getAgent(h)});
    }
    handles.sort(function (a,b) {
        if (a.handle < b.handle)
            return -1;
        if (a.handle > b.handle)
            return 1;
        return 0;
    });
    return handles;
}

// returns the handle of an agent by looking it up on the user's DHT entry, the last handle will be the current one?
function getHandle(userHash) {
    var handles = doGetLinkLoad(userHash,"handle");
    var n = handles.length -1;
    var h = handles[n];
    return (n >= 0) ? h.handle : "";
}

// returns the agent associated agent by converting the handle to a hash
// and getting that hash's source from the DHT
function getAgent(handle) {
    var handleHash = makeHash(handle);
    var sources = get(handleHash,{GetMask:HC.GetMask.Sources});

    if (isErr(sources)) {sources = [];}
    if (sources != undefined) {
        var n = sources.length -1;
        return (n >= 0) ? sources[n] : "";
    }
    return "";
}

function commitToss(initiator,initiatorSeed,responder,responderSeed,call) {
    var toss = {initiator:initiator,initiatorSeedHash:initiatorSeed,responder:responder,responderSeedHash:responderSeed,call:call};
    return commit("toss",JSON.stringify(toss));
}


function commitSeed() {
    var salt = ""+Math.random()+""+Math.random();
    return commit("seed",salt+"-"+Math.floor(Math.random()*10));
}

// initiates node2node communication with an agent to commit
// seeds values for the toss, followed by the toss entry itself afterwards
function requestToss(req) {
    var mySeed = commitSeed();
    var response = send(req.agent,{type:"tossReq",seed:mySeed});
    debug("requestToss response:"+response);
    response = JSON.parse(response);
    // create our own copy of the toss according to the seed and call from the responder
    var theToss = commitToss(App.Key.Hash,mySeed,req.agent,response.seed,response.call);
    if (theToss != response.toss) {
        return {error:"toss didn't match!"};
    }
    return {toss:theToss};
}

// listen for a toss request
function receive(from,msg) {
    var type = msg.type;
    if (type=='tossReq') {
        var mySeed = commitSeed();
        // call whether we want head or tails randomly.
        var call = Math.floor(Math.random()*10)%2 == 0;
        var theToss = commitToss(from,msg.seed,App.Key.Hash,mySeed,call);
        return {seed:mySeed,toss:theToss,call:call};
    } else if (type=="seedReq") {
        // make sure I committed toss and the seed hash is one of the seeds in the commit
        var rsp = get(msg.toss,{Local:true,GetMask:HC.GetMask.EntryType+HC.GetMask.Entry});
        if (!isErr(rsp) && rsp.EntryType == "toss") {
            var entry = JSON.parse(rsp.Entry.C);
            if (entry.initiatorSeedHash == msg.seedHash || entry.responderSeedHash == msg.seedHash) {
                // if so then I can reveal the seed
                var seed = get(msg.seedHash,{Local:true,GetMask:HC.GetMask.Entry});
                return seed.C;
            }
        }
    }
    return null;
}

function confirmSeed(seed,seedHash) {
    seed = JSON.parse(seed);
    var h = makeHash(seed);
    return (h == seedHash) ? seed :undefined;
}

// initiates node2node communication with an agent to retrieve the actual seed values
// after they were committed so we can find out what the toss actually was
function confirmToss(toss) {
    var rsp = get(toss,{GetMask:HC.GetMask.Sources+HC.GetMask.Entry+HC.GetMask.EntryType});
    if (!isErr(rsp) && rsp.EntryType == "toss") {
        var sources = rsp.Sources;
        var entry = JSON.parse(rsp.Entry.C);
        // check with the actual players in the record to get their seed values now that the
        // toss has been recorded publicly

        var iSeed = send(entry.initiator,{type:"seedReq",seedHash:entry.initiatorSeedHash,toss:toss});
        iSeed = confirmSeed(iSeed,entry.initiatorSeedHash);
        if (iSeed) {
            var rSeed = send(entry.responder,{type:"seedReq",seedHash:entry.responderSeedHash,toss:toss});
            rSeed = confirmSeed(rSeed,entry.responderSeedHash);
            if (rSeed) {
                var i = parseInt(iSeed.split("-")[1]);
                var r = parseInt(rSeed.split("-")[1]);
                // compare the odd evenness of the addition of the two seed values to the call
                var sum = (i+r);
                debug("call was:"+entry.call);
                debug("and sum of seed is:"+sum);
                var result = ((sum%2==0) == entry.call) ? "win" : "loss";
                debug("so responder gets a "+result);
                return result;
            }
        }
    } else {
        debug("confirmToss: error getting toss or bad type:"+JSON.stringify(rsp));
    }
    return "";
}

function winLose(str){
    // calculate hash of string
    var hash = 0;
    for (i = 0; i < str.length; i++) {
        char = str.charCodeAt(i);
        hash = char + (hash << 6) + (hash << 16) - hash;
    }
    return hash;
}

// ==============================================================================
// HELPERS: unexposed functions
// ==============================================================================


// helper function to resolve which has will be used as "me"
function getMe() {return App.Key.Hash;}

// helper function to resolve which hash will be used as the base for the directory
// currently we just use the DNA hash as our entry for linking the directory to
// TODO commit an anchor entry explicitly for this purpose.
function getDirectory() {return App.DNA.Hash;}


// helper function to actually commit a handle and its links on the directory
// this function gets called at genesis time only because all other times handle gets
// updated using newHandle
function addHandle(handle) {
    // TODO confirm no collision
    var key = commit("handle",handle);        // On my source chain, commits a new handle entry
    var me = getMe();
    var directory = getDirectory();

    debug(handle+" is "+key);

    commit("handle_links", {Links:[{Base:me,Link:key,Tag:"handle"}]});
    commit("directory_links", {Links:[{Base:directory,Link:key,Tag:"handle"}]});

    return key;
}

// helper function to determine if value returned from holochain function is an error
function isErr(result) {
    return ((typeof result === 'object') && result.name == "HolochainError");
}

// helper function to do getLink call, handle the no-link error case, and copy the returned entry values into a nicer array
function doGetLinkLoad(base, tag) {
    // get the tag from the base in the DHT
    var links = getLink(base, tag,{Load:true});
    if (isErr(links)) {
        links = [];
    } else {
        links = links.Links;
    }
    var links_filled = [];
    for (var i=0;i <links.length;i++) {
        var link = {H:links[i].H};
        link[tag] = links[i].E;
        links_filled.push(link);
    }
    debug("Links Filled:"+JSON.stringify(links_filled));
    return links_filled;
}

// helper function to call getLinks, handle the no links entry error, and build a simpler links array.
function doGetLink(base,tag) {
    // get the tag from the base in the DHT
    var links = getLink(base, tag,{Load:true});
    if (isErr(links)) {
        links = [];
    }
     else {
        links = links.Links;
    }
    debug("Links:"+JSON.stringify(links));
    var links_filled = [];
    for (var i=0;i <links.length;i++) {
        links_filled.push(links[i].H);
    }
    return links_filled;
}

// ==============================================================================
// CALLBACKS: Called by back-end system, instead of front-end app or UI
// ===============================================================================

// GENESIS - Called only when your source chain is generated:'hc gen chain <name>'
// ===============================================================================
function genesis() {                            // 'hc gen chain' calls the genesis function in every zome file for the app

    // use the agent string (usually email) used with 'hc init' to identify myself and create a new handle
    addHandle(App.Agent.String);
    //commit("anchor",{type:"sys",value:"directory"});
    return true;
}

// ===============================================================================
//   VALIDATION functions for *EVERY* change made to DHT entry -
//     Every DHT node uses their own copy of these functions to validate
//     any and all changes requested before accepting. put / mod / del & metas
// ===============================================================================

function validateCommit(entry_type,entry,header,pkg,sources) {
    debug("validate commit: "+entry_type);
    return validate(entry_type,entry,header,sources);
}

function validatePut(entry_type,entry,header,pkg,sources) {
    debug("validate put: "+entry_type);
    return validate(entry_type,entry,header,sources);
}

function validate(entry_type,entry,header,sources) {
    if (entry_type=="handle") {
        return true;
    }
    return true;
}

// Are there types of tags that you need special permission to add links?
// Examples:
//   - Only Bob should be able to make Bob a "follower" of Alice
//   - Only Bob should be able to list Alice in his people he is "following"
function validateLink(linkEntryType,baseHash,links,pkg,sources){
    debug("validate link: "+linkEntryType);
    if (linkEntryType=="handle_links") {
        var length = links.length;
        // a valid handle is when:

        // there should just be one or two links only
        if (length==2) {
            // if this is a modify it will have two links the first of which
            // will be the del and the second the new link.
            if (links[0].LinkAction != HC.LinkAction.Del) return false;
            if (links[1].LinkAction != HC.LinkAction.Add) return false;
        } else if (length==1) {
            // if this is a new handle, there will just be one Add link
            if (links[0].LinkAction != HC.LinkAction.Add) return false;
        } else {return false;}

        for (var i=0;i<length;i++) {
            var link = links[i];
            // the base must be this base
            if (link.Base != baseHash) return false;
            // the base must be the source
            if (link.Base != sources[0]) return false;
            // The tag name should be "handle"
            if (link.Tag != "handle") return false;
            //TODO check something about the link, i.e. get it and check it's type?
        }
        return true;
    }
    return true;
}
function validateMod(entry_type,entry,header,replaces,pkg,sources) {
    debug("validate mod: "+entry_type+" header:"+JSON.stringify(header)+" replaces:"+JSON.stringify(replaces));
    if (entry_type == "handle") {
        // check that the source is the same as the creator
        // TODO we could also check that the previous link in the type-chain is the replaces hash.
        var orig_sources = get(replaces,{GetMask:HC.GetMask.Sources});
        if (isErr(orig_sources) || orig_sources == undefined || orig_sources.length !=1 || orig_sources[0] != sources[0]) {return false;}

    }
    return true;
}
function validateDel(entry_type,hash,pkg,sources) {
    debug("validate del: "+entry_type);
    return true;
}

// ===============================================================================
//   PACKAGING functions for *EVERY* validation call for DHT entry
//     What data needs to be sent for each above validation function?
//     Default: send and sign the chain entry that matches requested HASH
// ===============================================================================

function validatePutPkg(entry_type) {return null;}
function validateModPkg(entry_type) { return null;}
function validateDelPkg(entry_type) { return null;}
function validateLinkPkg(entry_type) { return null;}
