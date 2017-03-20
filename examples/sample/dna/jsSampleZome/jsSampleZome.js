
expose("getProperty",HC.STRING);
function getProperty(x) {return property(x)};
expose("addOdd",HC.STRING);
function addOdd(x) {return commit("oddNumbers",x);}
expose("addProfile",HC.JSON);
function addProfile(x) {return commit("profile",x);}
function validatePut(entry_type,entry,header,sources) {
  return validate(entry_type,entry,header,sources);
}
function validateCommit(entry_type,entry,header,sources) {
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
function validatePutMeta(baseType,baseHash,ptrType,ptrHash,tag,sources){return true}
function genesis() {return true}
