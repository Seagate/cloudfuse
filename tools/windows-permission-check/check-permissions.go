package main

import (
	"fmt"
	"os"
	"os/user"
)

func main() {
	// examineUser()
	// username := ""
	// domain := ""
	// password := ""
	// token, err := winox.GetImpersonationToken(username, domain, password)
	// if err != nil {
	// 	fmt.Printf("Failed to get impersonation token for %s: %v\n", username, err)
	// 	return
	// }
	// see doc: https://learn.microsoft.com/en-us/windows/win32/secauthz/access-mask-format?redirectedfrom=MSDN
	// or even better: https://kb.netapp.com/onprem/ontap/da/NAS/What_are_NTFS_access_mask_flags_with_corresponding_user_permissions
	path := `C:\Users\465803\Desktop\cloudFuse\zyx.rtf`
	if len(os.Args) > 1 {
		path = os.Args[1]
	}
	// has, err := winox.UserHasPermission(token, syscall.GENERIC_WRITE, path)
	// if err != nil {
	// 	fmt.Printf("Error when checking write permissions on path %s: %v\n", path, err)
	// 	return
	// }
	// if has {
	// 	fmt.Println("Write: YES. This permission the user HAS.")
	// } else {
	// 	fmt.Println("Write: NO. This permission the user is DENIED.")
	// }
	// has, err = winox.UserHasPermission(token, syscall.GENERIC_READ, path)
	// if err != nil {
	// 	fmt.Printf("Error when checking read permissions on path %s: %v\n", path, err)
	// 	return
	// }
	// if has {
	// 	fmt.Println("Read:  YES. This permission the user HAS.")
	// } else {
	// 	fmt.Println("Read:  NO. This permission the user is DENIED.")
	// }
}

func examineUser() {
	user, err := user.Current()
	if err != nil {
		fmt.Println("Error while getting current user:", err)
		return
	}
	fmt.Printf("User %s with Uid %s and Gid %s belongs to these groups:\n", user.Name, user.Uid, user.Gid)
	groupIds, err := user.GroupIds()
	if err != nil {
		fmt.Println("Error while getting user groups:", err)
		return
	}
	for _, groupId := range groupIds {
		fmt.Println(groupId)
	}
}
