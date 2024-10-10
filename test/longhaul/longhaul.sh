SERVICE="cloudfuse"
SCRIPT="longhaul.sh"
WORKDIR="/home/cloudfuse/cloudfuse"

echo "Staring script"
if pgrep -x "$SERVICE" > /dev/null
then
        echo "Check existing run"
        #count=`ps -aux | grep $SCRIPT | wc -l`
        #echo "Existing run count  : $count"

        if [ -e "longhaul.lock" ]
        then
                echo "Script already running"
                echo "`date` :: Already running" >> $WORKDIR/longhaul.log
        else
                touch longhaul.lock
                echo "New script start"
                if [ `stat -c %s $WORKDIR/longhaul.log` -gt 10485760 ]
                then
                        echo "`date` :: Trimmed " > $WORKDIR/longhaul.log
                fi

                echo "`whoami` : `date` :: `$WORKDIR/cloudfuse --version` Starting test " >> $WORKDIR/longhaul.log

                mem=$(top -b -n 1 -p `pgrep -x cloudfuse` | tail -1)
                elap=$( ps -p `pgrep -x cloudfuse` -o etime | tail -1)
                echo $mem " :: " $elap >> $WORKDIR/longhaul.log

                echo "Delete old data"
                echo "`date` : Cleanup old test data" >> $WORKDIR/longhaul.log
                rm -rf /blob_mnt/kernel

                echo "Start test"
                echo "`date` : Building Kernel"  >> $WORKDIR/longhaul.log
                mkdir /blob_mnt/kernel
                $WORKDIR/build_kernel.sh /blob_mnt/kernel/ 6.10.2

                if [ $? -ne 0 ]; then
                  echo "`date` : Make Failed" >> $WORKDIR/longhaul.log
                fi
                echo "End test"
                echo "`date` : Kernel Build complete"  >> $WORKDIR/longhaul.log

                sleep 30
                echo "Cleanup post test"
                rm -rf /blob_mnt/test/*
                rm -rf /blob_mnt/kernel

                cp  $WORKDIR/longhaul.log  /blob_mnt/
                rm -rf longhaul.lock
        fi
else
        echo "Blobfuse not running"
        echo "`date` :: Re-Starting cloudfuse *******************" >> $WORKDIR/longhaul.log
        $WORKDIR/cloudfuse unmount all

        rm -rf /blob_mnt/*

        echo "Start cloudfuse"
        $WORKDIR/cloudfuse mount /blob_mnt --log-level=log_debug --log-file-path=$WORKDIR/cloudfuse.log --log-type=base --block-cache --container-name=longhaul

        sleep 2

        if [ `stat -c %s $WORKDIR/restart.log` -gt 10485760 ]
        then
                echo "`date` Trimmed " > $WORKDIR/restart.log
        fi
        echo "`date`: Restart : `$WORKDIR/cloudfuse --version`" >> $WORKDIR/restart.log

        echo "Send mail"
        # Send email that cloudfuse has crashed
        echo "Cloudfuse Failure" | mail -s "Cloudfuse Restart" -A $WORKDIR/restart.log -a "From: longhaul@cloudfuse.com"

        cp $WORKDIR/cloudfuse.log /blob_mnt/
        cp $WORKDIR/longhaul.log  /blob_mnt/
        cp $WORKDIR/restart.log /blob_mnt/
fi
