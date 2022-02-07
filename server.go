package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"os/exec"
	"sync"
	"time"
)

type Switches struct {
	dpi bool
	tor bool
	tor_dns bool
	all_list_tor bool
	masking bool
	global_tor bool
}

type SendSwitches struct {
	Dpi bool   `json:"dpi"`
	Tor bool   `json:"tor"`
	Tor_dns bool   `json:"tor_dns"`
	All_list_tor bool  `json:"all_list"`
	Masking bool `json:"masking"`
	Global_tor bool `json:"global_tor"`
}

type SendData struct {
	SendSwitches `json:"state"`
	Domains_array []string `json:"domains"`
	Subnets_array []string `json:"subnets"`
}

var sw = Switches{true, true, false, false, true, false}
var old_sw = Switches{false, false, false, false, false, false}
var updatedList bool = false
var iptablesConfigureIsDone bool = true
var iptablesSync sync.WaitGroup

func main() {

	startSettings()

	updateSwitches()

	http.HandleFunc("/", homepage)
	http.HandleFunc("/unblock/", unblock)
	http.HandleFunc("/switchstate/", switchState)
	// http.HandleFunc("/poweroff/", poweroff)
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))
	http.Handle("/images/", http.StripPrefix("/images/", http.FileServer(http.Dir("images"))))
	http.ListenAndServe(":8080", nil)

	fmt.Println("Server started!")
}

func startSettings() {
	fmt.Println("executing the command 'ipset -N tornet nethash'")
	cmd := exec.Command("ipset", "-N", "tornet", "nethash")
        err := cmd.Run()
        if err != nil {
            log.Println(err)
        }

	fmt.Println("executing the command 'ipset -N usertornet nethash'")
	cmd = exec.Command("ipset", "-N", "usertornet", "nethash")
		err = cmd.Run()
		if err != nil {
			log.Println(err)
		}
	go updateUserBlockedList()
}

func homepage(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		fmt.Println("GET /")
		tmpl, _ := template.ParseFiles("templates/index.html")

		var sendsw = SendSwitches{sw.dpi, sw.tor, sw.tor_dns, sw.all_list_tor, sw.masking, sw.global_tor}

		data := SendData{sendsw, getDomainsArray(), getSubnetsArray()}

		finalJson, err := json.Marshal(data)
		if err != nil {
			fmt.Println(err)
			return
		}
		fmt.Println(string(finalJson))

		tmpl.Execute(w, string(finalJson))

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func unblock(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()                     // Parses the request body
    domain := r.Form.Get("domain")
    subnet := r.Form.Get("subnet")

	if domain != "" {
		fmt.Println("Unblocking domain: " + domain)   // iptables -t nat -A OUTPUT -p tcp --syn -d rutracker.org -j REDIRECT --to-ports 9040
		saveDomain(domain)
		cmd := exec.Command("iptables", "-t", "nat", "-A", "OUTPUT", "-p", "tcp", "--syn", "-d", domain, "-j", "REDIRECT", "--to-ports", "9040")
        err := cmd.Run()
        if err != nil {
            log.Println(err)
        }
	}

	if subnet != "" {
		fmt.Println("Unblocking subnet: " + subnet)
		saveSubnet(subnet)
		cmd := exec.Command("ipset", "-A", "usertornet", subnet)
        err := cmd.Run()
        if err != nil {
            log.Println(err)
        }
	}
}

func poweroff(w http.ResponseWriter, r *http.Request)  {
	fmt.Println("executing the command 'poweroff'")
	err := exec.Command("poweroff").Run()
    if err != nil {
        log.Fatal(err)
    }
}

func switchState(w http.ResponseWriter, r *http.Request)  {
	r.ParseForm()
	dpiSwitch := r.Form.Get("dpi")
	torSwitch := r.Form.Get("tor")
	tordnsSwitch := r.Form.Get("tordns")
	allblockedSwitch := r.Form.Get("allblocked")
	masking := r.Form.Get("masking")
	globalTor := r.Form.Get("globaltor")

	if dpiSwitch != "" {
		fmt.Println("DPI switch on = " + dpiSwitch)
		if dpiSwitch == "true"{
			sw.dpi = true
		} else if dpiSwitch == "false"{
			sw.dpi = false
		}
	}

	if torSwitch != "" {
		fmt.Println("TOR switch on = " + torSwitch)
		if torSwitch == "true"{
			sw.tor = true
		} else if torSwitch == "false"{
			sw.tor = false
		}
	}

	if tordnsSwitch != "" {
		fmt.Println("TOR DNS switch on = " + tordnsSwitch)
		if tordnsSwitch == "true"{
			sw.tor_dns = true
		} else if tordnsSwitch == "false"{
			sw.tor_dns = false
		}
	}

	if allblockedSwitch != "" {
		fmt.Println("All blocked switch on = " + allblockedSwitch)
		if allblockedSwitch == "true"{
			sw.all_list_tor = true
		} else if allblockedSwitch == "false"{
			sw.all_list_tor = false
		}
	}

	if masking != "" {
		fmt.Println("Masking switch on = " + masking)
		if masking == "true"{
			sw.masking = true
		} else if masking == "false"{
			sw.masking = false
		}
	}

	if globalTor != "" {
		fmt.Println("Masking switch on = " + globalTor)
		if globalTor == "true"{
			sw.global_tor = true
		} else if globalTor == "false"{
			sw.global_tor = false
		}
	}

	updateSwitches()

}

func updateSwitches()  {

	var changeIndex uint8 = 0

	if sw.dpi == true && old_sw.dpi == false {
		go updateDpi("start")
		old_sw.dpi = sw.dpi
		changeIndex += 1
	} else if sw.dpi == false && old_sw.dpi == true {
		go updateDpi("stop")
		old_sw.dpi = sw.dpi
		changeIndex += 1
	}

	if sw.tor == true && old_sw.tor == false {
		go updateTor("start")
		old_sw.tor = sw.tor
		changeIndex += 1
	} else if sw.tor == false && old_sw.tor == true {
		go updateTor("stop")
		old_sw.tor = sw.tor
		changeIndex += 1
	}

	if sw.tor_dns == true && old_sw.tor_dns == false {
		go updateTorDns("start")
		old_sw.tor_dns = sw.tor_dns
		changeIndex += 1
	} else if sw.tor_dns == false && old_sw.tor_dns == true {
		go updateTorDns("stop")
		old_sw.tor_dns = sw.tor_dns
		changeIndex += 1
	}

	if sw.all_list_tor == true && old_sw.all_list_tor == false {
		go updateListTor("start")
		old_sw.all_list_tor = sw.all_list_tor
		changeIndex += 1
	} else if sw.all_list_tor == false && old_sw.all_list_tor == true {
		go updateListTor("stop")
		old_sw.all_list_tor = sw.all_list_tor
		changeIndex += 1
	}

	if sw.masking == true && old_sw.masking == false {
		go updateMasking("start")
		old_sw.masking = sw.masking
		changeIndex += 1
	} else if sw.masking == false && old_sw.masking == true {
		go updateMasking("stop")
		old_sw.masking = sw.masking
		changeIndex += 1
	}

	if sw.global_tor == true && old_sw.global_tor == false {
		go updateGlobalTor("start")
		old_sw.global_tor = sw.global_tor
		changeIndex += 1
	} else if sw.global_tor == false && old_sw.global_tor == true {
		go updateGlobalTor("stop")
		old_sw.global_tor = sw.global_tor
		changeIndex += 1
	}

	if changeIndex != 0 {
		go configurationWaitingRoom()
	}
}

func configurationWaitingRoom()  {
	if iptablesConfigureIsDone == false {
		for tr := true; tr; tr = !iptablesConfigureIsDone{
			time.Sleep(1 * time.Second) // Ожидание
			fmt.Println("Waiting for the configuration")
		}
	}
	fmt.Println("Configuring...")
	go configureIptables()
}

func updateDpi(state string)  {
	if state == "start" {
		fmt.Println("Starting DPI...")
	} else {
		fmt.Println("Stopping DPI...")
	}
}

func updateTor(state string)  {
	if state == "start" {
		fmt.Println("Starting TOR...")
	} else {
		fmt.Println("Stopping TOR...")
	}
}

func updateTorDns(state string)  {
	if state == "start" {
		fmt.Println("Starting TOR DNS...")
	} else {
		fmt.Println("Stopping TOR DNS...")
	}
}

func updateListTor(state string)  {
	if state == "start" {
		fmt.Println("Starting TOR List...")
		if updatedList == false {
			fmt.Println("Updating the list of blocked addresses")
			go UpdateBlockedList()
		}
	} else {
		fmt.Println("Stopping TOR List...")
	}
}

func updateMasking(state string) {
	if state == "start" {
		fmt.Println("Starting Masking...")
	} else {
		fmt.Println("Stopping Masking...")
	}
}

func updateGlobalTor(state string) {
	if state == "start" {
		fmt.Println("Starting Global Tor...")
	} else {
		fmt.Println("Stopping Global Tor...")
	}
}

func configureIptables()  {
	iptablesConfigureIsDone = false

	iptablesDelAll()

	iptablesSync.Add(1)
	go addDefaultIptables()

	if sw.dpi == true {
		iptablesSync.Add(1)
		go addDpi()
	}

	if sw.tor == true {
		iptablesSync.Add(1)
		go addUserTor()
		go updateUserDomainsList()
	}

	if sw.tor_dns == true {
		iptablesSync.Add(1)
		go addTorDns()
	} else {
		iptablesSync.Add(1)
		go addDefaultDns()
	}

	if sw.all_list_tor == true {
		iptablesSync.Add(1)
		go addTor()
	}

	if sw.masking == true {
		iptablesSync.Add(1)
		go addMasking()
	}

	if sw.global_tor == true {
		iptablesSync.Add(1)
		go addGlobalTor()
	}

	// Ожидание завершения работы горутин
	iptablesSync.Wait()

	iptablesConfigureIsDone = true
}

func addDefaultIptables()  {
	fmt.Println("executing the command '/bin/bash scripts/startDefaultIptables.sh'")
	err := exec.Command("/bin/bash", "scripts/startDefaultIptables.sh").Run()
    if err != nil {
        log.Fatal(err)
    }
	iptablesSync.Done()
}

func addUserTor() {
	fmt.Println("executing the command '/bin/bash scripts/startUserTor.sh'")
	err := exec.Command("/bin/bash", "scripts/startUserTor.sh").Run()
    if err != nil {
        log.Fatal(err)
    }
	iptablesSync.Done()
}

func iptablesDelAll() {
	// iptables -F
	fmt.Println("executing the command 'iptables -F'")
	err := exec.Command("iptables", "-F").Run()
    if err != nil {
        log.Fatal(err)
    }

	// iptables -t nat -F
	fmt.Println("executing the command 'iptables -t nat -F'")
	err = exec.Command("iptables", "-t", "nat", "-F").Run()
    if err != nil {
        log.Fatal(err)
    }
}

func addDpi()  {
	fmt.Println("executing the command '/bin/bash scripts/startDpi.sh'")
	err := exec.Command("/bin/bash", "scripts/startDpi.sh").Run()      ///    переписать    ///
    if err != nil {
        log.Println(err)
    }
	iptablesSync.Done()
}

func addTor() {
	// ON redirect sites to tor
	fmt.Println("executing the command '/bin/bash scripts/startTor.sh'")
	err := exec.Command("/bin/bash", "scripts/startTor.sh").Run()
    if err != nil {
        log.Fatal(err)
    }
	iptablesSync.Done()
}

func addTorDns()  {
	// ON redirect dns requests to tor
	fmt.Println("executing the command '/bin/bash scripts/startTorDns.sh'")
	err := exec.Command("/bin/bash", "scripts/startTorDns.sh").Run()
    if err != nil {
        log.Fatal(err)
    }
	iptablesSync.Done()
}

func addDefaultDns()  {
	fmt.Println("executing the command '/bin/bash scripts/startDefaultDns.sh'")
	err := exec.Command("/bin/bash", "scripts/startDefaultDns.sh").Run()
    if err != nil {
        log.Fatal(err)
    }
	iptablesSync.Done()
}

func addMasking() {
	fmt.Println("executing the command '/bin/bash scripts/Masking.sh'")
	err := exec.Command("/bin/bash", "scripts/Masking.sh").Run()
    if err != nil {
        log.Fatal(err)
    }
	iptablesSync.Done()
}

func addGlobalTor() {
	fmt.Println("executing the command '/bin/bash scripts/startGlobalTor.sh'")
	err := exec.Command("/bin/bash", "scripts/startGlobalTor.sh").Run()
    if err != nil {
        log.Fatal(err)
    }
	iptablesSync.Done()
}

func UpdateBlockedList(){
	updatedList = true
	fmt.Println("executing the command '/bin/bash scripts/getBlocked.sh'")
	err := exec.Command("/bin/bash", "scripts/getBlocked.sh").Run()
    if err != nil {
        log.Fatal(err)
    }

    file, err := os.Open("/tmp/lst/ipsum.lst")
    if err != nil {
		fmt.Println("I can't open the file /tmp/lst/ipsum.lst")
        log.Fatal(err)
    }
    defer file.Close()

    scanner := bufio.NewScanner(file)
    for scanner.Scan() {

        fmt.Println("ipset -A tornet " + scanner.Text())

        cmd := exec.Command("ipset", "-A", "tornet", scanner.Text())
        err := cmd.Run()
        if err != nil {
            log.Println(err)
        }
    }

	file, err = os.Open("/tmp/lst/subnet.lst")
    if err != nil {
		fmt.Println("I can't open the file /tmp/lst/subnet.lst")
        log.Fatal(err)
    }
    defer file.Close()

    scanner = bufio.NewScanner(file)
    for scanner.Scan() {

        fmt.Println("ipset -A tornet " + scanner.Text())

        cmd := exec.Command("ipset", "-A", "tornet", scanner.Text())
        err := cmd.Run()
        if err != nil {
            log.Println(err)
        }
    }
}

func updateUserBlockedList() {
	file, err := os.Open("subnets.list")
    if err != nil {
		fmt.Println("I can't open the file subnets.list")
        log.Fatal(err)
    }
    defer file.Close()

    scanner := bufio.NewScanner(file)
    for scanner.Scan() {

        fmt.Println("ipset -A usertornet " + scanner.Text())

        cmd := exec.Command("ipset", "-A", "usertornet", scanner.Text())
        err := cmd.Run()
        if err != nil {
            log.Println(err)
        }
    }
}

func updateUserDomainsList() {
	file, err := os.Open("domains.list")
    if err != nil {
		fmt.Println("I can't open the file domains.list")
        log.Fatal(err)
    }
    defer file.Close()

	scanner := bufio.NewScanner(file)
    for scanner.Scan() {
        fmt.Println("Unblocking domain: " + scanner.Text())   // iptables -t nat -A OUTPUT -p tcp --syn -d rutracker.org -j REDIRECT --to-ports 9040
		cmd := exec.Command("iptables", "-t", "nat", "-A", "OUTPUT", "-p", "tcp", "--syn", "-d", scanner.Text(), "-j", "REDIRECT", "--to-ports", "9040")
        err := cmd.Run()
        if err != nil {
            log.Println(err)
        }
    }
}

func saveDomain(domain string){

	domain = domain + "\n"

	file, err := os.OpenFile("domains.list", os.O_APPEND|os.O_WRONLY, 0600)
    if err != nil {
        file, err = os.Create("domains.list")
		if err != nil{
			log.Println("Unable to create file domains.list:", err) 
		}
		defer file.Close()
    }
    defer file.Close()

    file.WriteString(domain)
}

func getDomainsArray() []string{
	var finalArray []string
	file, err := os.Open("domains.list")
    if err != nil {
        log.Fatal(err)
    }
    defer file.Close()
	
	scanner := bufio.NewScanner(file)
    for scanner.Scan() {
		finalArray = append(finalArray, scanner.Text())
    }
	return finalArray
}


func saveSubnet(subnet string){

	subnet = subnet + "\n"

	file, err := os.OpenFile("subnets.list", os.O_APPEND|os.O_WRONLY, 0600)
    if err != nil {
        file, err = os.Create("subnets.list")
		if err != nil{
			log.Println("Unable to create file subnets.list:", err) 
		}
		defer file.Close()
    }
    defer file.Close()

    file.WriteString(subnet)
}

func getSubnetsArray() []string{
	var finalArray []string
	file, err := os.Open("subnets.list")
    if err != nil {
        log.Fatal(err)
    }
    defer file.Close()
	
	scanner := bufio.NewScanner(file)
    for scanner.Scan() {
		finalArray = append(finalArray, scanner.Text())
    }
	return finalArray
}
