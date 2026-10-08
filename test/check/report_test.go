package check

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	tableauapi "github.com/tableauio/tableau"
	"github.com/tableauio/tableau/log"
	"github.com/tableauio/tableau/log/core"
)

type recordingDriver struct {
	messages []string
}

func (*recordingDriver) Name() string               { return "checker-report-test" }
func (*recordingDriver) GetLevel(string) core.Level { return core.DebugLevel }
func (d *recordingDriver) Print(record *core.Record) {
	d.messages = append(d.messages, fmt.Sprintf(*record.Format, record.Args...))
}

// TestCustomErrorReport exercises the CLI reporting pattern with joined,
// single, and previously inspected failures from different checkers.
func TestCustomErrorReport(t *testing.T) {
	for _, tt := range []struct {
		lang string
		want string
	}{
		{"en", `check failed, see errors below:
[1] error[E0005]: custom check failed
Workbook: conf/server/AITutorial.xlsx
Worksheet: AITutorialConf
Reason: 异人列表不能为空，教学配置ID: 10005

[2] error[E0005]: custom check failed
Workbook: conf/server/AITutorial.xlsx
Worksheet: AITutorialConf
Reason: 异人列表不能为空，教学配置ID: 10003

[3] error[E0005]: custom check failed
Workbook: conf/server/AITutorial.xlsx
Worksheet: AITutorialConf
Reason: 异人列表不能为空，教学配置ID: 10004

[4] error[E0005]: custom check failed
Workbook: conf/server/Pvp/Arena.xlsx
Worksheet: CommonConf
Reason: MaxRecentCnt must be greater than 0

[5] error[E0005]: custom check failed
Workbook: conf/server/Task.xlsx
Worksheet: TaskConfig
Reason: 任务集[赛季日常]中的任务[10100016]没配条件目标

[6] error[E0005]: custom check failed
Workbook: conf/server/Task.xlsx
Worksheet: TaskConfig
Reason: 任务集[赛季日常]中的任务[10100015]没配条件目标
`},
		{"zh", `check failed, see errors below:
[1] error[E0005]: custom check failed
工作簿: conf/server/AITutorial.xlsx
工作表: AITutorialConf
错误原因: 异人列表不能为空，教学配置ID: 10005

[2] error[E0005]: custom check failed
工作簿: conf/server/AITutorial.xlsx
工作表: AITutorialConf
错误原因: 异人列表不能为空，教学配置ID: 10003

[3] error[E0005]: custom check failed
工作簿: conf/server/AITutorial.xlsx
工作表: AITutorialConf
错误原因: 异人列表不能为空，教学配置ID: 10004

[4] error[E0005]: custom check failed
工作簿: conf/server/Pvp/Arena.xlsx
工作表: CommonConf
错误原因: MaxRecentCnt must be greater than 0

[5] error[E0005]: custom check failed
工作簿: conf/server/Task.xlsx
工作表: TaskConfig
错误原因: 任务集[赛季日常]中的任务[10100016]没配条件目标

[6] error[E0005]: custom check failed
工作簿: conf/server/Task.xlsx
工作表: TaskConfig
错误原因: 任务集[赛季日常]中的任务[10100015]没配条件目标
`},
	} {
		t.Run(tt.lang, func(t *testing.T) {
			require.NoError(t, tableauapi.SetLang(tt.lang))
			t.Cleanup(func() { require.NoError(t, tableauapi.SetLang("en")) })
			driver := &recordingDriver{}
			log.SetDriver(driver)
			t.Cleanup(func() { log.SetDriver(nil) })

			causes := []error{
				errors.New("异人列表不能为空，教学配置ID: 10005"),
				errors.New("异人列表不能为空，教学配置ID: 10003"),
				errors.New("异人列表不能为空，教学配置ID: 10004"),
				errors.New("MaxRecentCnt must be greater than 0"),
				errors.New("任务集[赛季日常]中的任务[10100016]没配条件目标"),
				errors.New("任务集[赛季日常]中的任务[10100015]没配条件目标"),
			}
			ai := tableauapi.WrapKV(errors.Join(causes[:3]...),
				tableauapi.KeyBookName, "conf/server/AITutorial.xlsx", tableauapi.KeySheetName, "AITutorialConf")
			arena := tableauapi.WrapKV(causes[3],
				tableauapi.KeyBookName, "conf/server/Pvp/Arena.xlsx", tableauapi.KeySheetName, "CommonConf")
			task := tableauapi.Inspect(tableauapi.WrapKV(errors.Join(causes[4:]...),
				tableauapi.KeyBookName, "conf/server/Task.xlsx", tableauapi.KeySheetName, "TaskConfig"))
			hub := NewHub()
			hub.checkers = map[string]checker{
				"AITutorialConf":  &failingChecker{failure: ai},
				"ArenaCommonConf": &failingChecker{failure: arena},
				"TaskConf":        &failingChecker{failure: task},
			}
			err := errors.Join(hub.check(0)...)
			serr := tableauapi.Inspect(err)
			require.Len(t, serr.Details, 6)
			reported := fmt.Errorf("check failed, see errors below:\n%w", serr)
			assert.Equal(t, tt.want, reported.Error())
			for i, cause := range causes {
				require.ErrorIs(t, reported, cause)
				assert.Equal(t, cause.Error(), serr.Details[i].Message)
				assert.Equal(t, "E0005", serr.Details[i].Code)
			}
			assert.Equal(t, []string{
				"=== RUN   AITutorialConf", "--- FAIL: AITutorialConf",
				"=== RUN   ArenaCommonConf", "--- FAIL: ArenaCommonConf",
				"=== RUN   TaskConf", "--- FAIL: TaskConf",
			}, driver.messages, "progress logs must not repeat the detailed error report")
		})
	}
}
