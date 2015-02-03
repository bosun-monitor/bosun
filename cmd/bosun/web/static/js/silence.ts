interface ISilenceScope extends ng.IScope {
	silences: any;
	error: string;
	start: string;
	end: string;
	duration: string;
	alert: string;
	hosts: string;
	tags: string;
	edit: string;
	testSilences: any;
	test: () => void;
	confirm: () => void;
	clear: (id: string) => void;
	change: () => void;
	disableConfirm: boolean;
	time: (v: any) => string;
	forget: string;
}

bosunControllers.controller('SilenceCtrl', ['$scope', '$http', '$location', '$route', function($scope: ISilenceScope, $http: ng.IHttpService, $location: ng.ILocationService, $route: ng.route.IRouteService) {
	var search = $location.search();
	$scope.start = search.start;
	$scope.end = search.end;
	$scope.duration = search.duration;
	$scope.alert = search.alert;
	$scope.hosts = search.hosts;
	$scope.tags = search.tags;
	$scope.edit = search.edit;
	$scope.forget = search.forget;
	if (!$scope.end && !$scope.duration) {
		$scope.duration = '1h';
	}
	function get() {
		$http.get('/api/silence/get')
			.success((data) => {
				$scope.silences = data;
			})
			.error((error) => {
				$scope.error = error;
			});
	}
	get();
	function getData() {
		var tags = ($scope.tags || '').split(',');
		if ($scope.hosts) {
			tags.push('host=' + $scope.hosts.split(/[ ,|]+/).join('|'));
		}
		tags = tags.filter((v) => { return v != ""; });
		var data: any = {
			start: $scope.start,
			end: $scope.end,
			duration: $scope.duration,
			alert: $scope.alert,
			tags: tags.join(','),
			edit: $scope.edit,
			forget: $scope.forget ? 'true' : null,
		};
		return data;
	}
	var any = search.start || search.end || search.duration || search.alert || search.hosts || search.tags || search.forget;
	var state = getData();
	$scope.change = () => {
		$scope.disableConfirm = true;
	};
	if (any) {
		$scope.error = null;
		$http.post('/api/silence/set', state)
			.success((data) => {
				if (!data) {
					data = {'(none)': false};
				}
				$scope.testSilences = data;
			})
			.error((error) => {
				$scope.error = error;
			});
	}
	$scope.test = () => {
		$location.search('start', $scope.start || null);
		$location.search('end', $scope.end || null);
		$location.search('duration', $scope.duration || null);
		$location.search('alert', $scope.alert || null);
		$location.search('hosts', $scope.hosts || null);
		$location.search('tags', $scope.tags || null);
		$location.search('forget', $scope.forget || null);
		$route.reload();
	};
	$scope.confirm = () => {
		$scope.error = null;
		$scope.testSilences = null;
		state.confirm = 'true';
		$http.post('/api/silence/set', state)
			.error((error) => {
				$scope.error = error;
			})
			.finally(get);
	};
	$scope.clear = (id: string) => {
		if (!window.confirm('Clear this silence?')) {
			return;
		}
		$scope.error = null;
		$http.post('/api/silence/clear', { id: id })
			.error((error) => {
				$scope.error = error;
			})
			.finally(get);
	};
	$scope.time = (v: any) => {
		var m = moment(v);
		return m.format();
	};
}]);
