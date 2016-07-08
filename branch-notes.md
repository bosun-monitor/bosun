Thoughts:
 - There might be certain aspects of the config that are not 'reloadable'. The main thing we want to reload is alert/template/lookup/macro etc configuration, not the actual system params. I think it is fine if the system params require a restart. If going this route I should reoganize the conf Struct to make it clear which are alert config and which are system config. Maybe this is configuration items realted to the schedule, vs those that are not.
 - This might be a good time to make the config an interface so the ground work for not text based configuration is in place?
 - What sort of signals can I used so this still works with Windows?
  - SigUSR2 won't build on windows. I played with fsnotify but it seems wonky - i.e. it tries to reload the file before the writing of the file is complete when I edit it with vm.
  - From https://golang.org/pkg/os/, "The only signal values guaranteed to be present on all systems are Interrupt (send the process an interrupt) and Kill (force the process to exit)." So maybe I need to change Int to be reload and kill?
 - Will I need to reload the web based schedule as well? 
   - Seems I can just repoint the schedule pointer to the new schedule

Main:

 - We parse the config
 - create url to listen to based on the config
 - init the metadata server // shouldn't need to be done for a reload
 - set skipLast which tells redis not to load last datapoints // should be remembered for config reloads? OR just not hit?
 - Tell the sched package to `Load`
 - If the relay is enabled, run a go routine to run the realy with a mux handler // should not need to be done on reload unless the param changed
 - If a tsdbhost is set, start sending self data to it and launch a test server for the put path to prevent data from being relayed to opentsdb. Also set the conf tsdb host (not sure why?)
 - Set up the Internet Proxy ... ? Not sure what this does yet, will look at code
 - If command like quiet flag, set the conf to be quiet
 - as long as the nocheck flag is not present, start the schedule by calling `sched.run`. This is the main thing I'm going to need to trace
 - Make a interrupt handler, it calls sched.Close(). Make that does some of the work we need already
 - Do watch stuff for development 
 - select {}

Schedule Load and Init:
 - The package has a Default schedule that is a struct tied to the scope of the package
 - Load uses this package scoped schedule when Load is called
 - Load calls Init on the schedule passing it the configuration
   - Init initalized the following parts of the schedule:
     - The schedule `Conf`, which is just a pointer to the conf.Conf (arg)
     - the scheule `Group`, a map of time to alert keys, not sure what this is used for yet...
     - A structure of pendingUnknowns .. notifications I think?
     - `lastLogTimes` - Don't know what this is yet, alertkey -> time.Time used for what?
     - sets `LastCheck` to `utcNow()` which is `time.Now().UTC()`
     - sets `ctx` to a new `checkContext` which has the time and a new query cache
     - sets the schedules data access up. In the case of Redis is sets up a connection, but in the case of Ledis of starts a ledis server -- dammit - will have to think about ledis with reloads
     - creates a new search for the schedule using the newly created data access
     - ugh something with bolt go away `s.db, err = bolt.Open(c.StateFile, 0600, nil)`
 - Load also finally calls restoreStatem which I think is just for migrating old bolt stuff to Redis

Schedule Run:
 - Make sure we have a configuration
 - Create the nc chan - not sure what this is yet, notifications?
 - start pinging hosts if that is enabled in the config. This can be moved somewhere earlier? Make not even attached to the schedule.
 - Kick of a couple goroutines - `dispatchNotifications()` and `updateCheckContext`, will have to go see what they do....
 - Here is the meat, it ranges over the configured alerts in `a.Conf.Alerts` and spins of a goroutine for each by passing that alert conf to schedules `RunAlert` method.
   - Run Alert is a loop that waits the calculated check duration, and calls the schedules's `checkAlert(a)` method and then sets the schedules last check to `utcNow()`

Schedule Close:
 - It just persists the Last Data

