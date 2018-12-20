package ftp

// FTP status codes, defined in RFC 959 https://tools.ietf.org/html/rfc959
const (
	StatusInitiating    = 100
	StatusRestartMarker = 110
	StatusReadyMinute   = 120
	StatusAlreadyOpen   = 125
	StatusAboutToSend   = 150

	StatusCommandOK             = 200
	StatusCommandNotImplemented = 202
	StatusSystem                = 211
	StatusDirectory             = 212
	StatusFile                  = 213
	StatusHelp                  = 214
	StatusName                  = 215
	StatusReady                 = 220
	StatusClosing               = 221
	StatusDataConnectionOpen    = 225
	StatusClosingDataConnection = 226
	StatusPassiveMode           = 227
	StatusLongPassiveMode       = 228
	StatusExtendedPassiveMode   = 229
	StatusLoggedIn              = 230
	StatusLoggedOut             = 231
	StatusLogoutAck             = 232
	StatusRequestedFileActionOK = 250
	StatusPathCreated           = 257

	StatusUserOK             = 331
	StatusLoginNeedAccount   = 332
	StatusRequestFilePending = 350

	StatusNotAvailable             = 421
	StatusCanNotOpenDataConnection = 425
	StatusTransfertAborted         = 426
	StatusInvalidCredentials       = 430
	StatusHostUnavailable          = 434
	StatusFileActionIgnored        = 450
	StatusActionAborted            = 451
	Status452                      = 452

	StatusBadCommand              = 500
	StatusBadArguments            = 501
	StatusNotImplemented          = 502
	StatusBadSequence             = 503
	StatusNotImplementedParameter = 504
	StatusNotLoggedIn             = 530
	StatusStorNeedAccount         = 532
	StatusFileUnavailable         = 550
	StatusPageTypeUnknown         = 551
	StatusExceededStorage         = 552
	StatusBadFileName             = 553
)
