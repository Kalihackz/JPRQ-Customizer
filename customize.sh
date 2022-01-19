#!/bin/bash

clear

banner() {

echo "      _ ____  ____   ___  ____    __  __           _ 
     | |  _ \|  _ \ / _ \/ ___|  |  \/  | ___   __| |
  _  | | |_) | |_) | | | \\___ \  | |\\/| |/ _ \\ / _\` |
 | |_| |  __/|  _ <| |_| |___) | | |  | | (_) | (_| |
  \\___/|_|   |_| \\_\\\\__\\_\\____/  |_|  |_|\\___/ \\__,_|
                                                     "| lolcat -p 1
echo -e "                            - A JPRQS Modifier for all\n"| lolcat -p 1
echo -e "              	                    By Kalihackz\n"| lolcat -p 1

}

banner

echo -e "\n[*] Run time: $(date) @ $(hostname)\n" | lolcat -p 1

echo -n "Enter domain name : "
read domain

if [ -d "LocalExpose" ] 
then
    rm -rf LocalExpose
fi

mkdir LocalExpose

cp -r TunnelServer/* LocalExpose 

echo -e "\nGenerating correct main.go file\n" | lolcat -p 1
printf "#######                 (33%%)\r" | lolcat -a --duration 5 -s 6 -p 1
printf "##############          (66%%)\r" | lolcat -a --duration 5 -s 6 -p 1
printf "##################      (85%%)\r" | lolcat -a --duration 5 -s 6 -p 1
printf "#####################   (100%%)\r" | lolcat -a --duration 5 -s 6 -p 1
PATH=`find TunnelServer -name main.go`
MAIN_VALUE=$(<"$PATH")
generated_data=${MAIN_VALUE/yunik.com.np/$domain}
cd LocalExpose
echo "$generated_data">'main.go'
echo -e "\n\nDone\nInfo to be used -\n* Use subdomain name as : $USER\n* Copy contents of LocalExpose Directory to your Server\n* go run main.go\n"