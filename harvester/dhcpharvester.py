#!/usr/bin/python
import sys, threading, thread, subprocess, re, time, datetime, bisect, BaseHTTPServer
from optparse import OptionParser
from Queue import Queue

def parse_timestamp(raw_str):
    tokens = raw_str.split()

    if len(tokens) == 1:
        if tokens[0].lower() == 'never':
            return 'never';

        else:
            raise Exception('Parse error in timestamp')

    elif len(tokens) == 3:
        return datetime.datetime.strptime(' '.join(tokens[1:]),
            '%Y/%m/%d %H:%M:%S')

    else:
        raise Exception('Parse error in timestamp')

def timestamp_is_ge(t1, t2):
    if t1 == 'never':
        return True

    elif t2 == 'never':
        return False

    else:
        return t1 >= t2


def timestamp_is_lt(t1, t2):
    if t1 == 'never':
        return False

    elif t2 == 'never':
        return t1 != 'never'

    else:
        return t1 < t2


def timestamp_is_between(t, tstart, tend):
    return timestamp_is_ge(t, tstart) and timestamp_is_lt(t, tend)


def parse_hardware(raw_str):
    tokens = raw_str.split()

    if len(tokens) == 2:
        return tokens[1]

    else:
        raise Exception('Parse error in hardware')


def strip_endquotes(raw_str):
    return raw_str.strip('"')


def identity(raw_str):
    return raw_str


def parse_binding_state(raw_str):
    tokens = raw_str.split()

    if len(tokens) == 2:
        return tokens[1]

    else:
        raise Exception('Parse error in binding state')


def parse_next_binding_state(raw_str):
    tokens = raw_str.split()

    if len(tokens) == 3:
        return tokens[2]

    else:
        raise Exception('Parse error in next binding state')


def parse_rewind_binding_state(raw_str):
    tokens = raw_str.split()

    if len(tokens) == 3:
        return tokens[2]

    else:
        raise Exception('Parse error in next binding state')

def parse_res_fixed_address(raw_str):
    return raw_str

def parse_res_hardware(raw_str):
    tokens = raw_str.split()
    return tokens[1]

def parse_reservation_file(res_file):
    valid_keys = {
        'hardware'      : parse_res_hardware,
        'fixed-address' : parse_res_fixed_address,
    }

    res_db = {}
    res_rec = {}
    in_res = False
    for line in res_file:
        if line.lstrip().startswith('#'):
            continue
        tokens = line.split()

        if len(tokens) == 0:
            continue

        key = tokens[0].lower()

        if key == 'host':
            if not in_res:
                res_rec = {'hostname' : tokens[1]}
                in_res = True

            else:
                raise Exception("Parse error in reservation file")
        elif key == '}':
            if in_res:
                for k in valid_keys:
                    if callable(valid_keys[k]):
                        res_rec[k] = res_rec.get(k, '')
                    else:
                        res_rec[k] = False

                hostname = res_rec['hostname']

                if hostname in res_db:
                    res_db[hostname].insert(0, res_rec)

                else:
                    res_db[hostname] = [res_rec]

                res_rec = {}
                in_res = False

            else:
                raise Exception('Parse error in reservation file')

        elif key in valid_keys:
            if in_res:
                value = line[(line.index(key) + len(key)):]
                value = value.strip().rstrip(';').rstrip()

                if callable(valid_keys[key]):
                    res_rec[key] = valid_keys[key](value)
                else:
                    res_rec[key] = True

            else:
                raise Exception('Parse error in reservation file')

        else:
            if in_res:
                raise Exception('Parse error in reservation file')

    if in_res:
        raise Exception('Parse error in reservation file')

    # Turn the leases into an array
    results = []
    for res in res_db:
        results.append({
            'client-hostname'   : res_db[res][0]['hostname'],
            'hardware'          : res_db[res][0]['hardware'],
            'ip_address'        : res_db[res][0]['fixed-address'],
        }) 
    return results
        

def parse_leases_file(leases_file):
    valid_keys = {
        'starts':           parse_timestamp,
        'ends':         parse_timestamp,
        'tstp':         parse_timestamp,
        'tsfp':         parse_timestamp,
        'atsfp':        parse_timestamp,
        'cltt':         parse_timestamp,
        'hardware':         parse_hardware,
        'binding':          parse_binding_state,
        'next':         parse_next_binding_state,
        'rewind':           parse_rewind_binding_state,
        'uid':          strip_endquotes,
        'client-hostname':      strip_endquotes,
        'option':           identity,
        'set':          identity,
        'on':           identity,
        'abandoned':        None,
        'bootp':        None,
        'reserved':         None,
        }

    leases_db = {}

    lease_rec = {}
    in_lease = False
    in_failover = False

    for line in leases_file:
        if line.lstrip().startswith('#'):
            continue

        tokens = line.split()

        if len(tokens) == 0:
            continue

        key = tokens[0].lower()

        if key == 'lease':
            if not in_lease:
                ip_address = tokens[1]

                lease_rec = {'ip_address' : ip_address}
                in_lease = True

            else:
                raise Exception('Parse error in leases file')

        elif key == 'failover':
            in_failover = True
        elif key == '}':
            if in_lease:
                for k in valid_keys:
                    if callable(valid_keys[k]):
                        lease_rec[k] = lease_rec.get(k, '')
                    else:
                        lease_rec[k] = False

                ip_address = lease_rec['ip_address']

                if ip_address in leases_db:
                    leases_db[ip_address].insert(0, lease_rec)

                else:
                    leases_db[ip_address] = [lease_rec]

                lease_rec = {}
                in_lease = False

            elif in_failover:
                in_failover = False
                continue
            else:
                raise Exception('Parse error in leases file')

        elif key in valid_keys:
            if in_lease:
                value = line[(line.index(key) + len(key)):]
                value = value.strip().rstrip(';').rstrip()

                if callable(valid_keys[key]):
                    lease_rec[key] = valid_keys[key](value)
                else:
                    lease_rec[key] = True

            else:
                raise Exception('Parse error in leases file')

        else:
            if in_lease:
                raise Exception('Parse error in leases file')

    if in_lease:
        raise Exception('Parse error in leases file')

    return leases_db


def round_timedelta(tdelta):
    return datetime.timedelta(tdelta.days,
        tdelta.seconds + (0 if tdelta.microseconds < 500000 else 1))


def timestamp_now():
    n = datetime.datetime.utcnow()
    return datetime.datetime(n.year, n.month, n.day, n.hour, n.minute,
        n.second)# + (0 if n.microsecond < 500000 else 1))


def lease_is_active(lease_rec, as_of_ts):
    return lease_rec['binding'] != 'free' and timestamp_is_between(as_of_ts, lease_rec['starts'],
        lease_rec['ends'])


def ipv4_to_int(ipv4_addr):
    parts = ipv4_addr.split('.')
    return (int(parts[0]) << 24) + (int(parts[1]) << 16) + \
        (int(parts[2]) << 8) + int(parts[3])

def select_active_leases(leases_db, as_of_ts):
    retarray = []
    sortedarray = []

    for ip_address in leases_db:
        lease_rec = leases_db[ip_address][0]

        if lease_is_active(lease_rec, as_of_ts):
            ip_as_int = ipv4_to_int(ip_address)
            insertpos = bisect.bisect(sortedarray, ip_as_int)
            sortedarray.insert(insertpos, ip_as_int)
            retarray.insert(insertpos, lease_rec)

    return retarray

def matched(list, target):
    if list == None:
        return False

    for r in list:
        if re.match(r, target) != None:
            return True
    return False

def convert_to_seconds(time_val):
    num = int(time_val[:-1])
    if time_val.endswith('s'):
        return num
    elif time_val.endswith('m'):
        return num * 60
    elif time_val.endswith('h'):
        return num * 60 * 60
    elif time_val.endswith('d'):
        return num * 60 * 60 * 24

def ping(ip, timeout):
    cmd = ['ping', '-c', '1', '-w', timeout, ip]
    try:
        out = subprocess.check_output(cmd)
        return True
    except subprocess.CalledProcessError as e:
        return False

def ping_worker(list, to, respQ):
    for lease in list:
        respQ.put(
            {
                'verified': ping(lease['ip_address'], to),
                'lease' : lease,
            })

def interruptable_get(q):
    r = None
    while True:
        try:
            return q.get(timeout=1000)
        except Queue.Empty:
            pass

##############################################################################

def harvest(options):

    ifilter = None
    if options.include != None:
        ifilter = options.include.translate(None, ' ').split(',')

    rfilter = None
    if options.filter != None:
        rfilter = options.filter.split(',')

    myfile = open(options.leases, 'r')
    leases = parse_leases_file(myfile)
    myfile.close()

    reservations = []
    try:
        with open(options.reservations, 'r') as res_file:
            reservations = parse_reservation_file(res_file)
        res_file.close()
    except (IOError) as e:
        pass
    
    now = timestamp_now()
    report_dataset = select_active_leases(leases, now) + reservations

    verified = []
    if options.verify:

        # To verify is lease information is valid, i.e. that the host which got the lease still responding
        # we ping the host. Not perfect, but good for the main use case. As the lease file can get long
        # a little concurrency is used. The lease list is divided amoung workers and each worker takes
        # a share.
        respQ = Queue()
        to = str(convert_to_seconds(options.timeout))
        share = int(len(report_dataset) / options.worker_count)
        extra = len(report_dataset) % options.worker_count
        start = 0
        for idx in range(0, options.worker_count):
            end = start + share
            if extra > 0:
                end = end + 1
                extra = extra - 1
            worker = threading.Thread(target=ping_worker, args=(report_dataset[start:end], to, respQ))
            worker.daemon = True
            worker.start()
            start = end

        # All the verification work has been farmed out to worker threads, so sit back and wait for reponses.
        # Once all responses are received we are done. Probably should put a time out here as well, but for
        # now we expect a response for every lease, either positive or negative
        count = 0
        while count != len(report_dataset):
            resp = interruptable_get(respQ)
            count = count + 1
            if resp['verified']:
                print("INFO: verified host '%s' with address '%s'" % (resp['lease']['client-hostname'], resp['lease']['ip_address']))
                verified.append(resp['lease'])
            else:
                print("INFO: dropping host '%s' with address '%s' (not verified)" % (resp['lease']['client-hostname'], resp['lease']['ip_address']))
    else:
        verified = report_dataset

    # Look for duplicate names and add the compressed MAC as a suffix
    names = {}
    for lease in verified:
        # If no client hostname use MAC
        name = lease['client-hostname']
        if 'client-hostname' not in lease or len(name) == 0:
            name = "UNK-" + lease['hardware'].translate(None, ':').upper()

        if name in names:
            names[name] = '+'
        else:
            names[name] = '-'

    size = 0
    count = 0
    for lease in verified:
        name = lease['client-hostname']
        if 'client-hostname' not in lease or len(name) == 0:
            name = "UNK-" + lease['hardware'].translate(None, ':').upper()

        if (ifilter != None and name in ifilter) or matched(rfilter, name):
            if names[name] == '+':
                lease['client-hostname'] = name + '-' + lease['hardware'].translate(None, ':').upper()
            size = max(size, len(lease['client-hostname']))
            count += 1

    if options.dest == '-':
        out=sys.stdout
    else:
        out=open(options.dest, 'w+')

    for lease in verified:
        name = lease['client-hostname']
        if 'client-hostname' not in lease or len(name) == 0:
            name = "UNK-" + lease['hardware'].translate(None, ':').upper()

        if ifilter != None and name in ifilter or matched(rfilter, name):
            out.write(format(name, '<'+str(size)) + ' IN A ' + lease['ip_address'] + '\n')
    if options.dest != '-':
        out.close()
    return count

def reload_zone(rndc, server, port, key, zone):
    cmd = [rndc, '-s', server]
    if key != None:
        cmd.extend(['-c', key])
    cmd.extend(['-p', port, 'reload'])
    if zone != None:
        cmd.append(zone)

    try:
        out = subprocess.check_output(cmd)
        print("INFO: [%s UTC] updated DNS sever" % time.asctime(time.gmtime()))
    except subprocess.CalledProcessError as e:
        print("ERROR: failed to update DNS server, exit code %d" % e.returncode)
        print(e.output)

def handleRequestsUsing(requestQ):
  return lambda *args: ApiHandler(requestQ, *args)

class ApiHandler(BaseHTTPServer.BaseHTTPRequestHandler):
    def __init__(s, requestQ, *args):
       s.requestQ = requestQ
       BaseHTTPServer.BaseHTTPRequestHandler.__init__(s, *args)

    def do_HEAD(s):
        s.send_response(200)
        s.send_header("Content-type", "application/json")
        s.end_headers()

    def do_POST(s):
        if s.path == '/harvest':
            waitQ = Queue()
            s.requestQ.put(waitQ)
            resp = waitQ.get(block=True, timeout=None)
            s.send_response(200)
            s.send_header('Content-type', 'application/json')
            s.end_headers()

            if resp == "QUIET":
                s.wfile.write('{ "response" : "QUIET" }')
            else:
                s.wfile.write('{ "response" : "OK" }')

        else:
            s.send_response(404)

    def do_GET(s):
        """Respond to a GET request."""
        s.send_response(404)

def do_api(hostname, port, requestQ):
    server_class = BaseHTTPServer.HTTPServer
    httpd = server_class((hostname, int(port)), handleRequestsUsing(requestQ))
    print("INFO: [%s UTC] Start API server on %s:%s" % (time.asctime(time.gmtime()), hostname, port))
    try:
        httpd.serve_forever()
    except KeyboardInterrupt:
        pass
    httpd.server_close()
    print("INFO: [%s UTC] Stop API server on %s:%s" % (time.asctime(time.gmtime()), hostname, port))

def harvester(options, requestQ):
    quiet = convert_to_seconds(options.quiet)
    last = -1
    resp = "OK"
    while True:
        responseQ = requestQ.get(block=True, timeout=None)
        if last == -1 or (time.time() - last) > quiet:
            work_field(options)
            last = time.time()
            resp = "OK"
        else:
            resp = "QUIET"

        if responseQ != None:
            responseQ.put(resp)

def work_field(options):
    start = datetime.datetime.now()
    print("INFO: [%s UTC] starting to harvest hosts from DHCP" % (time.asctime(time.gmtime())))
    count = harvest(options)
    end = datetime.datetime.now()
    delta = end - start
    print("INFO: [%s UTC] harvested %d hosts, taking %d seconds" % (time.asctime(time.gmtime()), count, delta.seconds))
    if options.update:
        reload_zone(options.rndc, options.server, options.port, options.key, options.zone)

def main():
    parser = OptionParser()
    parser.add_option('-l', '--leases', dest='leases', default='/dhcp/dhcpd.leases',
        help="specifies the DHCP lease file from which to harvest")
    parser.add_option('-x', '--reservations', dest='reservations', default='/etc/dhcp/dhcpd.reservations',
        help="specified the reservation file as ISC DHCP doesn't update the lease file for fixed addresses")
    parser.add_option('-d', '--dest', dest='dest', default='/bind/dhcp_harvest.inc',
        help="specifies the file to write the additional DNS information")
    parser.add_option('-i', '--include', dest='include', default=None,
        help="list of hostnames to include when harvesting DNS information")
    parser.add_option('-f', '--filter', dest='filter', default=None,
        help="list of regex expressions to use as an include filter")
    parser.add_option('-r', '--repeat', dest='repeat', default=None,
        help="continues to harvest DHCP information every specified interval")
    parser.add_option('-c', '--command', dest='rndc', default='rndc',
        help="shell command to execute to cause reload")
    parser.add_option('-k', '--key', dest='key', default=None,
        help="rndc key file to use to access DNS server")
    parser.add_option('-s', '--server', dest='server', default='127.0.0.1',
        help="server to reload after generating updated dns information")
    parser.add_option('-p', '--port', dest='port', default='954',
        help="port on server to contact to reload server")
    parser.add_option('-z', '--zone', dest='zone', default=None,
        help="zone to reload after generating updated dns information")
    parser.add_option('-u', '--update', dest='update', default=False, action='store_true',
        help="update the DNS server, by reloading the zone")
    parser.add_option('-y', '--verify', dest='verify', default=False, action='store_true',
        help="verify the hosts with a ping before pushing them to DNS")
    parser.add_option('-t', '--timeout', dest='timeout', default='1s',
        help="specifies the duration to wait for a verification ping from a host")
    parser.add_option('-a', '--apiserver', dest='apiserver', default='0.0.0.0',
        help="specifies the interfaces on which to listen for API requests")
    parser.add_option('-e', '--apiport', dest='apiport', default='8954',
        help="specifies the port on which to listen for API requests")
    parser.add_option('-q', '--quiet', dest='quiet', default='1m',
        help="specifieds a minimum quiet period between actually harvest times.")
    parser.add_option('-w', '--workers', dest='worker_count', type='int', default=5,
        help="specifies the number of workers to use when verifying IP addresses")

    (options, args) = parser.parse_args()

    # Kick off a thread to listen for HTTP requests to force a re-evaluation
    requestQ = Queue()
    api = threading.Thread(target=do_api, args=(options.apiserver, options.apiport, requestQ))
    api.daemon = True
    api.start()

    if options.repeat == None:
        work_field(options)
    else:
        secs = convert_to_seconds(options.repeat)
        farmer = threading.Thread(target=harvester, args=(options, requestQ))
        farmer.daemon = True
        farmer.start()
        while True:
            cropQ = Queue()
            requestQ.put(cropQ)
            interruptable_get(cropQ)
            time.sleep(secs)

if __name__ == "__main__":
    main()
