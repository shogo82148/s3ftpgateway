package ftp

// FTP status codes, defined in RFC 959 https://tools.ietf.org/html/rfc959
const (
	StatusInitiating    = 100
	StatusRestartMarker = 110 // Restart marker reply.
	StatusReadyMinute   = 120 // Service ready in nnn minutes.
	StatusAlreadyOpen   = 125 // Data connection already open; transfer starting.
	StatusAboutToSend   = 150 // File status okay; about to open data connection.

	StatusCommandOK             = 200 // Command okay.
	StatusCommandNotImplemented = 202 // Command not implemented, superfluous at this site.
	StatusSystem                = 211 // System status, or system help reply.
	StatusDirectory             = 212 // Directory status.
	StatusFile                  = 213 // File status.
	StatusHelp                  = 214 // Help message.
	StatusName                  = 215 // NAME system type.
	StatusReady                 = 220 // Service ready for new user.
	StatusClosing               = 221 // Service closing control connection.
	StatusDataConnectionOpen    = 225 // Data connection open; no transfer in progress.
	StatusClosingDataConnection = 226 // Closing data connection.
	StatusPassiveMode           = 227 // Entering Passive Mode
	StatusLongPassiveMode       = 228
	StatusExtendedPassiveMode   = 229
	StatusLoggedIn              = 230 // User logged in, proceed.
	StatusRequestedFileActionOK = 250 // Requested file action okay, completed.
	StatusPathCreated           = 257 // "PATHNAME" created.

	StatusUserOK             = 331 // User name okay, need password.
	StatusLoginNeedAccount   = 332 // Need account for login.
	StatusRequestFilePending = 350 // Requested file action pending further information.

	StatusNotAvailable             = 421 // Service not available, closing control connection.
	StatusCanNotOpenDataConnection = 425 // Can't open data connection.
	StatusTransfertAborted         = 426 // Connection closed; transfer aborted.
	StatusInvalidCredentials       = 430
	StatusHostUnavailable          = 434
	StatusFileActionIgnored        = 450 // Requested file action not taken.
	StatusActionAborted            = 451 // Requested action aborted: local error in processing.
	StatusInsufficientStorageSpace = 452 // Requested action not taken. Insufficient storage space in system.

	StatusBadCommand              = 500 // Syntax error, command unrecognized.
	StatusBadArguments            = 501 // Syntax error in parameters or arguments.
	StatusNotImplemented          = 502 // Command not implemented.
	StatusBadSequence             = 503 // Bad sequence of commands.
	StatusNotImplementedParameter = 504 // Command not implemented for that parameter.
	StatusNotLoggedIn             = 530 // Not logged in.
	StatusStorNeedAccount         = 532 // Need account for storing files.
	StatusFileUnavailable         = 550 // Requested action not taken. File unavailable (e.g., file not found, no access).
	StatusPageTypeUnknown         = 551 // Requested action aborted: page type unknown.
	StatusExceededStorage         = 552 // Requested file action aborted. Exceeded storage allocation.
	StatusBadFileName             = 553 // Requested action not taken. File name not allowed.
)

// Extra FTP Status codes defined in RFC 2228 https://tools.ietf.org/html/rfc2228
const (
	StatusLoggedInSecured                    = 232 // User logged in, authorized by security data exchange.
	StatusSecurityDataExchangeComplete       = 234 // Security data exchange complete.
	StatusSecurityDataExchangeSuccess        = 235 // the security data exchange completed successfully.
	StatusSecurityMechanismOK                = 334 // the requested security mechanism is ok.
	StatusSecurityDataAcceptable             = 335 // the security data is acceptable.
	StatusUserOKNeedChallenge                = 336 // Username okay, need password.  Challenge is "...."
	StatusNeedSomeUnavailableResource        = 431 // Need some unavailable resource to process security.
	StatusCommandProtectionLevelDenied       = 533 // Command protection level denied for policy reasons.
	StatusRequestDenied                      = 534 // Request denied for policy reasons.
	StatusFailedSecurityCheck                = 535 // Failed security check (hash, sequence, etc).
	StatusProtLevelNotSupported              = 536 // Requested PROT level not supported by mechanism.
	StatusCommandProtectionLevelNotSupported = 537 // Command protection level not supported by security mechanism.
	StatusSafeReply                          = 631 // Integrity protected reply.
	StatusPrivateReply                       = 632 // Confidentiality and integrity protected reply.
	StatusConfidentialReply                  = 633 // Confidentiality protected reply.
)
