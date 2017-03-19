expose("getProperty",HC.STRING);
function getProperty(x) {return property(x)};
expose("addOdd",HC.STRING);
function addOdd(x) {return commit("myOdds",x);}
expose("addProfile",HC.JSON);
function addProfile(x) {return commit("profile",x);}
function validate(entry_type,entry,props) {
if (entry_type=="myOdds") {
  return entry%2 != 0
}
if (entry_type=="profile") {
  return true
}
return false
}
function genesis() {return true}
