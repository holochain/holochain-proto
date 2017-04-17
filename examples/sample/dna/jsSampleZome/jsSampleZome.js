
function testStrFn1(x) {return "result: "+x};
function testStrFn2(x){ return parseInt(x)+2};
function testJsonFn1(x){ x.output = x.input*2; return x;};
function testJsonFn2(x){ return [{a:'b'}] };

function getProperty(x) {return property(x)};
function addOdd(x) {return commit("oddNumbers",x);}
function addProfile(x) {return commit("profile",x);}
function validatePut(entry_type,entry,header,sources) {
  return validate(entry_type,entry,header,sources);
}
function validateDel(entry_type,hash,sources) {
  return true;
}
function validateCommit(entry_type,entry,header,sources) {
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
  return false
}
function validateLink(linkEntryType,baseHash,linkHash,tag,sources){return true}
function genesis() {return true}
