package evidence

import "errors"

// 业务规则错误，HTTP 层据此映射状态码。
var (
	ErrNotFound            = errors.New("对象不存在")
	ErrAlreadyExists       = errors.New("对象已存在")
	ErrInvalid             = errors.New("参数不合法")
	ErrMappingNotApproved  = errors.New("映射尚未获批，不能进入正式比较")
	ErrMappingNotPending   = errors.New("映射已作出审批结论，不能再次审批")
	ErrMissingPrerequisite = errors.New("先修课程版本不存在")
	ErrFrameworkMissing    = errors.New("课程引用的能力框架不存在")
	ErrSystemMismatch      = errors.New("映射两端必须属于不同制度背景")
	ErrObjectionMissing    = errors.New("驳回需附上书面异议理由")
	ErrCalibrationChanged  = errors.New("统计口径已变更，不得沿用旧口径生成结论")
	ErrCalibrationMissing  = errors.New("试点未绑定统计口径")
	ErrSampleIncomplete    = errors.New("样本不完整，不得生成成效结论")
	ErrDefinitionChanged   = errors.New("指标定义与当前口径不一致，不得生成结论")
	ErrIndicatorNotInScope = errors.New("观察值包含口径外指标")
	ErrPilotMismatch       = errors.New("上报数据的院校或教师群体不在试点范围")
)
