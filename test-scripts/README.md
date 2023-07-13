This folder contains scripts to run the end to end tests easily
on your machine if you need to run them locally rather than just
running on the CI/CD pipeline.

To run on Linux follow the following steps:

1. First go into the helper folder and run
   ./setup.sh
   
   This will create a file that stores environment variables for the
   tests. Some of the variables store locations where the tests will
   be mounted and a cache folder. In particular the folders at
   ~/e2e-test  and  ~/e2e-temp are assumed to exist. So you will need
   to create these files. Additionally it assumed that the directory
   where lyvecloudfuse is installed is at ~/lyvecloudfuse. If your
   git repo is at a different folder then edit this variable in this
   file var.env (the file is included in .gitignore so don't worry about
   editing it).

2. Ensure that you have a config.yaml file in the lyvecloudfuse 
   repository. These tests will run with that config file.

3. Run the test by running the scripts. You must be in the test-scripts
   directory to execute them. You can run them like

   ./benchmark-test.sh

   ./e2e-test.sh

   ./mount-test.sh

   ./stress-test.sh


   In particular the e2e-test and mount-test are the most useful of the
   scripts to run. The benchmark-test file runs a benchmark if we want
   to measure the speed of downloads. And stress tests creates a lot
   of files at various sizes to test the system. I would not recommend
   running these tests unless there is a specific need to.

   stress-test.sh and e2e-test.sh also have special options to run
   longer tests. If you want to enable these check the options in the files
   and change them to either true or false. For example, -quick-test=true
   is the default for e2e-test.sh but if you want to run longer tests
   you can change this to false. Do note that the longer tests generate a
   large amount of data, so don't run them frequently as that will bring
   our data charges up.

To run on Windows follow the following steps:

1. First go into the directory in the helper folder and run
   .\setup.ps1

   This will create a file that stores environment variables for the
   tests. Some of the variables store locations where the tests will
   be mounted and a cache folder. The default to mount the directory is
   in the Z: directory. Also thethe folder at ~/e2e-temp is assumed to exist. 
   So you will need to create these or change them to ones you prefer. 
   Additionally it assumed that the directory where lyvecloudfuse is 
   installed is at ~\lyvecloudfuse. If your git repo is at a different 
   folder then edit this variable in this file var.env (the file is included 
   in .gitignore so don't worry about editing it).

2. When running the e2e tests you will need to open a separate terminal and mount lyvecloudfuse
   into the folder that you reference in the e2e-test_windows.ps1 file.

3. Run the test file using a powershell terminal on Windows. You can run them like

   .\e2e-test_windows.ps1

   .\mount-test_windows.ps1
