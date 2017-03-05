expose("addOdd",_STRING);
function addOdd(x) {return commit("myOdds",x);}
function validate(entry_type,entry,meta) {
if (entry_type=="myOdds") {
  return entry%2 != 0
}
return false
}
function init() {
    return true
}
