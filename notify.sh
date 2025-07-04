#!/busybox sh
### NZBGET POST-PROCESSING SCRIPT

# Replace this with the URL of your API
API_URL="http://momenarr:3000/api/notify"

# You can add more parameters if needed
POST_DATA="{
    \"name\": \"${NZBPP_NZBNAME}\",
    \"category\": \"${NZBPP_CATEGORY}\",
    \"trakt\": \"${NZBPR_TRAKT}\",
    \"dir\": \"${NZBPP_DIRECTORY}\",
    \"status\": \"${NZBPP_TOTALSTATUS}\"
}"

# Send notification to the API
/busybox wget --header="Content-Type: application/json" --post-data="${POST_DATA}" "${API_URL}"

# Exit with status 93
exit 93
