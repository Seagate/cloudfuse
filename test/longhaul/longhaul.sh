
SERVICE="lyvecloudfuse"
SCRIPT="longhaul.sh"

# To create ramdisk
# sudo mount -t tmpfs -o rw,size=4G tmpfs /mnt/ramdisk

cd /home/vibhansa/go/src/azure-storage-fuse/

if pgrep -x "$SERVICE" > /dev/null
then
	if pgrep -x "$SCRIPT" > /dev/null
	then
		echo "`date` :: Already running" >> ./longhaul2.log
	else
		if [ `stat -c %s ./longhaul2.log` -gt 10485760 ]
		then 
			echo "`date` :: Trimmed " > ./longhaul2.log
		fi

		echo "`whoami` : `date` :: `./lyvecloudfuse --version` Starting stress test " >> ./longhaul2.log

		mem=$(top -b -n 1 -p `pgrep -x lyvecloudfuse` | tail -1)
		elap=$( ps -p `pgrep -x lyvecloudfuse` -o etime | tail -1)
		echo $mem " :: " $elap >> ./longhaul2.log
	
		rm -rf /home/vibhansa/blob_mnt2/stress	
		rm -rf /home/vibhansa/blob_mnt2/myfile*
		
		#go test -timeout 120m -v ./test/stress_test/stress_test.go -args -mnt-path=/home/vibhansa/blob_mnt2 -quick=false 2&> ./stress.log
		./test/longhaul/stresstest.sh
		echo "`whoami` : `date` :: Ending stress test " >> ./longhaul2.log
		cp  ./longhaul2.log  /home/vibhansa/blob_mnt2/
		cp ./stress.log /home/vibhansa/blob_mnt2/
		
		sleep 30

		rm -rf /mnt/ramdisk/*
		rm -rf /home/vibhansa/blob_mnt2/stress	
		sudo rm -rf /var/log/blob*.gz
	fi
else
	echo "`date` :: Re-Starting lyvecloudfuse *******************" >> ./longhaul2.log
	rm -rf /home/vibhansa/blob_mnt2/*
	rm -rf /mnt/ramdisk/*
	sudo fusermount -u ~/blob_mnt2
	rm -rf /mnt/ramdisk2/*
	./lyvecloudfuse mount ~/blob_mnt2 --config-file=./config.yaml
	sleep 2

	if [ `stat -c %s ./restart2.log` -gt 10485760 ]
	then 
		echo "`date` Trimmed " > ./restart2.log
	fi
	echo "`date`: Restart : `./lyvecloudfuse --version`" >> ./restart2.log

	# Send email that lyvecloudfuse has crashed
	echo "Lyvecloudfuse Failure" | mail -s "Lyvecloudfuse Restart" -A ./restart2.log -a "From: longhaul@blobfuse.com" <mail id here>
	
	cp /var/log/lyvecloudfuse.log /home/vibhansa/blob_mnt2/
	cp ./longhaul2.log  /home/vibhansa/blob_mnt2/
	cp ./restart2.log /home/vibhansa/blob_mnt2/
fi	
