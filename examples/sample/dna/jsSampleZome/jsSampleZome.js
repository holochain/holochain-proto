
function unexposed(x) {return x+" fish";};
function testStrFn1(x) {return "result: "+x};
function testStrFn2(x){ return parseInt(x)+2};
function testJsonFn1(x){ x.output = x.input*2; return x;};
function testJsonFn2(x){ return [{a:'b'}] };

function getProperty(x) {return property(x)};
function addOdd(x) {return commit("oddNumbers",x);}
function addProfile(x) {return commit("profile",x);}
function validatePut(entry_type,entry,header,pkg,sources) {
  return validate(entry_type,entry,header,sources);
}
function validateMod(entry_type,entry,header,replaces,pkg,sources) {
  return true;
}
function validateDel(entry_type,hash,pkg,sources) {
  return true;
}
function validateCommit(entry_type,entry,header,pkg,sources) {
  if (entry_type == "rating") {return true}
  return validate(entry_type,entry,header,sources);
}
function validate(entry_type,entry,header,sources) {
  if (entry_type=="oddNumbers") {
    return entry%2 != 0
  }
  if (entry_type=="profile") {
    return true
  }
  if (entry_type=="secret") {
    return true
  }
  return false
}
function validateLink(linkEntryType,baseHash,linkHash,tag,pkg,sources){return true}
function validatePutPkg(entry_type) {
  req = {};
  req[HC.PkgReq.Chain]=HC.PkgReq.ChainOpt.Full;
  return req;
}
function validateModPkg(entry_type) { return null}
function validateDelPkg(entry_type) { return null}
function validateLinkPkg(entry_type) { return null}

function genesis() {return true}

function receive(from,message) {
  // send back a pong message of what came in the ping message!
  return {pong:message.ping}
}

