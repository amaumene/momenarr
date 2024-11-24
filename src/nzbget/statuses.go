package nzbget

// DeleteStatus determine if and why a download was removed.
type DeleteStatus string

// DeleteStatuses go here.
//
//nolint:lll
const (
	DeleteNONE   DeleteStatus = "NONE"   // not deleted;
	DeleteMANUAL DeleteStatus = "MANUAL" // the download was manually deleted by user;
	DeleteHEALTH DeleteStatus = "HEALTH" // the download was deleted by health check;
	DeleteDUPE   DeleteStatus = "DUPE"   // the download was deleted by duplicate check;
	DeleteBAD    DeleteStatus = "BAD"    // v14.0 the download was marked as BAD by a queue//script during download;
	DeleteSCAN   DeleteStatus = "SCAN"   // v16.0 the download was deleted because the nzb//file could not be parsed (malformed nzb//file);
	DeleteCOPY   DeleteStatus = "COPY"   // v16.0 the download was deleted by duplicate check because an nzb//file with exactly same content exists
)

// GroupStatus determines the current status of a download group.
type GroupStatus string

// GroupStatuses go here.
//
//nolint:lll
const (
	GroupQUEUED            GroupStatus = "QUEUED"             // queued for download;
	GroupPAUSED            GroupStatus = "PAUSED"             // paused;
	GroupDOWNLOADING       GroupStatus = "DOWNLOADING"        // item is being downloaded;
	GroupFETCHING          GroupStatus = "FETCHING"           // nzbGroupStatus = "" //file is being fetched from URL (Kind=URL);
	GroupPPQUEUED          GroupStatus = "PP_QUEUED"          // queued for postGroupStatus = "" //processing (completely downloaded);
	GroupLOADINGPARS       GroupStatus = "LOADING_PARS"       // stage of parGroupStatus = "" //check;
	GroupVERIFYINGSOURCES  GroupStatus = "VERIFYING_SOURCES"  // stage of parGroupStatus = "" //check;
	GroupREPAIRING         GroupStatus = "REPAIRING"          // stage of parGroupStatus = "" //check;
	GroupVERIFYINGREPAIRED GroupStatus = "VERIFYING_REPAIRED" // stage of parGroupStatus = "" //check;
	GroupRENAMING          GroupStatus = "RENAMING"           // processed by parGroupStatus = "" //renamer;
	GroupUNPACKING         GroupStatus = "UNPACKING"          // being unpacked;
	GroupMOVING            GroupStatus = "MOVING"             // moving files from intermediate directory into destination directory;
	GroupEXECUTINGSCRIPT   GroupStatus = "EXECUTING_SCRIPT"   // executing postGroupStatus = "" //processing script;
	GroupPPFINISHED        GroupStatus = "PP_FINISHED"        // postGroupStatus = "" //processing is finished, the item is about to be moved to history.
)

// ParStatus determines the result of par-check/repair.
type ParStatus string

// ParStatuses go here.
//
//nolint:lll
const (
	ParNONE           ParStatus = "NONE"            // par-check wasn’t performed;
	ParFAILURE        ParStatus = "FAILURE"         // par-check has failed;
	ParREPAIRPOSSIBLE ParStatus = "REPAIR_POSSIBLE" // download is damaged, additional par-files were downloaded but the download was not repaired. Either the option ParRepair is disabled or the par-repair was cancelled by option ParTimeLimit;
	ParSUCCESS        ParStatus = "SUCCESS"         // par-check was successful;
	ParMANUAL         ParStatus = "MANUAL"          // download is damaged but was not checked/repaired because option ParCheck is set to Manual.
)

// ExParStatus determines if the download was repaired using duplicate par-scan mode.
type ExParStatus string

// ExParStatuses go here.
const (
	ExParRECIPIENT ExParStatus = "RECIPIENT" // repaired using blocks from other duplicates;
	ExParDONOR     ExParStatus = "DONOR"     // has donated blocks to repair another duplicate;
)

// UnpackStatus determines the result of the unpack extraction.
type UnpackStatus string

// UnpackStatuses go here.
//
//nolint:lll
const (
	UnpackNONE     UnpackStatus = "NONE"     // unpack wasn’t performed, either no archive files were found or the unpack is disabled for that download or globally;
	UnpackFAILURE  UnpackStatus = "FAILURE"  // unpack has failed;
	UnpackSPACE    UnpackStatus = "SPACE"    // unpack has failed due to not enough disk space;
	UnpackPASSWORD UnpackStatus = "PASSWORD" // unpack has failed because the password was not provided or was wrong. Only for rar5-archives;
	UnpackSUCCESS  UnpackStatus = "SUCCESS"  // unpack was successful.
)

// URLStatus determines the result of the URL download.
type URLStatus string

// URLStatuses go here.
//
//nolint:lll
const (
	URLNONE        URLStatus = "NONE"         // that nzb-file were not fetched from an URL;
	URLSUCCESS     URLStatus = "SUCCESS"      // that nzb-file was fetched from an URL;
	URLFAILURE     URLStatus = "FAILURE"      // the fetching of the URL has failed.
	URLSCANSKIPPED URLStatus = "SCAN_SKIPPED" // The URL was fetched successfully but downloaded file was not nzb-file and was skipped by the scanner;
	URLSCANFAILURE URLStatus = "SCAN_FAILURE" // The URL was fetched successfully but an error occurred during scanning of the downloaded file. The downloaded file isn’t a proper nzb-file. This status usually means the web-server has returned an error page (HTML page) instead of the nzb-file.
)

// MoveStatus determines the result of moving files from intermediate directory into final directory.
type MoveStatus string

// MoveStatuses go here.
//
//nolint:lll
const (
	MoveNONE    MoveStatus = "NONE"    // the moving wasn’t made because either the option InterDir is not in use or the par-check or unpack have failed;
	MoveSUCCESS MoveStatus = "SUCCESS" // files were moved successfully;
	MoveFAILURE MoveStatus = "FAILURE" // the moving has failed.
)

// MoveStatus determines if the download was marked by a user.
type MarkStatus string

// MarkStatuses go here.
const (
	MarkNONE MarkStatus = "NONE" // not marked;
	MarkGOOD MarkStatus = "GOOD" // the download was marked as good by user using command Mark as good in history dialog;
	MarkBAD  MarkStatus = "BAD"  // the download was marked as bad by user using command Mark as bad in history dialog;
)

// ScriptStatus determines the result of a post-processing script.
type ScriptStatus string

// ScriptStatuses go here.
const (
	ScriptNONE    ScriptStatus = "NONE"
	ScriptFAILURE ScriptStatus = "FAILURE"
	ScriptSUCCESS ScriptStatus = "SUCCESS"
)

// LogKind determines the kind of log entry.
type LogKind string

// LogKinds go here.
const (
	LogINFO    LogKind = "INFO"
	LogWARNING LogKind = "WARNING"
	LogERROR   LogKind = "ERROR"
	LogDETAIL  LogKind = "DETAIL"
	LogDEBUG   LogKind = "DEBUG" // only if compiled in debug mode.
)
