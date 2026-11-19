package funcs

import (
	"fmt"
	"github.com/cihub/seelog"
	"github.com/smartping/smartping/src/g"
)

//clear timeout alert table
func ClearArchive() {
	seelog.Info("[func:ClearArchive] ", "starting run ClearArchive ")
	modifier := fmt.Sprintf("-%d day", g.Cfg.Base["Archive"])
	g.DLock.Lock()
	g.Db.Exec("DELETE FROM alertlog WHERE logtime < date('now','start of day', ?)", modifier)
	g.Db.Exec("DELETE FROM mappinglog WHERE logtime < date('now','start of day', ?)", modifier)
	g.Db.Exec("DELETE FROM pinglog WHERE logtime < date('now','start of day', ?)", modifier)
	g.Db.Exec("DELETE FROM tcppinglog WHERE logtime < date('now','start of day', ?)", modifier)
	g.DLock.Unlock()
	seelog.Info("[func:ClearArchive] ", "ClearArchive Finish ")
}
