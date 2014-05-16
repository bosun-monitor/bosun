interface IHostScope extends ng.IScope {
	cpu: any;
	host: string;
	time: string;
	tab: string;
	metrics: string[];
	mlink: (m: string) => Request;
	setTab: (m: string) => void;
	idata: any;
	fs: any;
	fsdata: any;
	fs_current: any;
	mem: any;
	mem_total: number;
	interfaces: string[];
	error: string;
	running: string;
	filterMetrics: string;
}

tsafControllers.controller('HostCtrl', ['$scope', '$http', '$location', '$route', function($scope: IHostScope, $http: ng.IHttpService, $location: ng.ILocationService, $route: ng.route.IRouteService) {
	var search = $location.search();
	$scope.host = search.host;
	$scope.time = search.time;
	$scope.tab = search.tab || "stats";
	$scope.idata = [];
	$scope.fsdata = [];
	$scope.fs_current = [];
	$scope.metrics = [];
	$scope.mlink = (m: string) => {
		var r = new Request();
		var q = new Query();
		q.metric = m;
		q.tags = { 'host': $scope.host };
		r.queries.push(q);
		return r;
	};
	$scope.setTab = function(t: string) {
		$location.search('tab', t);
		$scope.tab = t;
	};
	$http.get('/api/metric/host/' + $scope.host)
		.success(function(data: string[]) {
			$scope.metrics = data || [];
		});
	var cpu_r = new Request();
	cpu_r.start = $scope.time;
	cpu_r.queries = [
		new Query({
			metric: 'os.cpu',
			derivative: 'counter',
			tags: { host: $scope.host },
		})
	];
	var width = 500;
	$http.get('/api/graph?' + 'json=' + encodeURIComponent(JSON.stringify(cpu_r)) + '&autods=' + width)
		.success((data) => {
			data.Series[0].name = 'Percent Used';
			$scope.cpu = data.Series;
		});
	$http.get('/api/tagv/iface/os.net.bytes?host=' + $scope.host)
		.success((data) => {
			$scope.interfaces = data;
			angular.forEach($scope.interfaces, function(i) {
				var net_bytes_r = new Request();
				net_bytes_r.start = $scope.time;
				net_bytes_r.queries = [
					new Query({
						metric: "os.net.bytes",
						rate: true,
						tags: { host: $scope.host, iface: i, direction: "*" },
					})
				];
				$http.get('/api/graph?' + 'json=' + encodeURIComponent(JSON.stringify(net_bytes_r)) + '&autods=' + width)
					.success((data) => {
						angular.forEach(data.Series, function(d) {
							d.data = d.data.map((dp: any) => { return { x: dp.x, y: dp.y * 8 } });
							if (d.name.indexOf("direction=out") != -1) {
								d.data = d.data.map((dp: any) => { return { x: dp.x, y: dp.y * -1 } });
								d.name = "out";
							} else {
								d.name = "in";
							}
						});
						$scope.idata[$scope.interfaces.indexOf(i)] = { name: i, data: data.Series };
					});
			});
		});
	$http.get('/api/tagv/disk/os.disk.fs.space_total?host=' + $scope.host)
		.success((data) => {
			$scope.fs = data;
			angular.forEach($scope.fs, function(i) {
				if (i == '/dev/shm') {
					return;
				}
				var fs_r = new Request();
				fs_r.start = $scope.time;
				fs_r.queries.push(new Query({
					metric: "os.disk.fs.space_total",
					tags: { host: $scope.host, disk: i },
				}));
				fs_r.queries.push(new Query({
					metric: "os.disk.fs.space_used",
					tags: { host: $scope.host, disk: i },
				}));
				$http.get('/api/graph?' + 'json=' + encodeURIComponent(JSON.stringify(fs_r)) + '&autods=' + width)
					.success((data) => {
						data.Series[1].name = "Used";
						$scope.fsdata[$scope.fs.indexOf(i)] = { name: i, data: [data.Series[1]] };
						var total: number = Math.max.apply(null, data.Series[0].data.map((d: any) => { return d.y; }));
						var c_val: number = data.Series[1].data.slice(-1)[0].y;
						var percent_used: number = c_val / total * 100;
						$scope.fs_current[$scope.fs.indexOf(i)] = {
							total: total,
							c_val: c_val,
							percent_used: percent_used,
						};
					});
			});
		});
	var mem_r = new Request();
	mem_r.start = $scope.time;
	mem_r.queries.push(new Query({
		metric: "os.mem.total",
		tags: { host: $scope.host },
	}));
	mem_r.queries.push(new Query({
		metric: "os.mem.used",
		tags: { host: $scope.host },
	}));
	$http.get('/api/graph?' + 'json=' + encodeURIComponent(JSON.stringify(mem_r)) + '&autods=' + width)
		.success((data) => {
			data.Series[1].name = "Used";
			$scope.mem_total = Math.max.apply(null, data.Series[0].data.map((d: any) => { return d.y; }));
			$scope.mem = [data.Series[1]];
		});
}]);