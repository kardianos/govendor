package help

type HelpMessage byte

const (
	MsgNone HelpMessage = iota
	MsgFull
	MsgInit
	MsgList
	MsgAdd
	MsgUpdate
	MsgRemove
	MsgFetch
	MsgStatus
	MsgSync
	MsgMigrate
	MsgGet
	MsgLicense
	MsgShell
	MsgGovendorLicense
	MsgGovendorVersion
)

func (msg HelpMessage) String() string {
	msgText := ""
	switch msg {
	default:
		panic("Unknown message type")
	case MsgNone:
	case MsgFull:
		msgText = helpFull
	case MsgInit:
		msgText = helpInit
	case MsgList:
		msgText = helpList
	case MsgAdd:
		msgText = helpAdd
	case MsgUpdate:
		msgText = helpUpdate
	case MsgRemove:
		msgText = helpRemove
	case MsgFetch:
		msgText = helpFetch
	case MsgStatus:
		msgText = helpStatus
	case MsgSync:
		msgText = helpSync
	case MsgMigrate:
		msgText = helpMigrate
	case MsgGet:
		msgText = helpGet
	case MsgLicense:
		msgText = helpLicense
	case MsgShell:
		msgText = helpShell
	case MsgGovendorLicense:
		msgText = msgGovendorLicenses
	case MsgGovendorVersion:
		msgText = msgGovendorVersion
	}
	return msgText
}
