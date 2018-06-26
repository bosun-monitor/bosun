
/// <reference path="0-bosun.ts" />
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
	user: string;
	message: string;
	getEditSilenceLink: (silence: any, silenceId: string) => string;
}

bosunControllers.controller('SilenceCtrl', ['$scope', '$http', '$location', '$route', 'linkService', function($scope: ISilenceScope, $http: ng.IHttpService, $location: ng.ILocationService, $route: ng.route.IRouteService, linkService: ILinkService) {
	var search = $location.search();
	$scope.start = search.start;
	$scope.end = search.end;
	$scope.duration = search.duration;
	$scope.alert = search.alert;
	$scope.hosts = search.hosts;
	$scope.tags = search.tags;
	$scope.edit = search.edit;
	$scope.forget = search.forget;
	$scope.message = search.message;
	if (!$scope.end && !$scope.duration) {
		$scope.duration = '1h';
	}
	function filter(data: any[], startBefore: any, startAfter: any, endAfter: any, endBefore: any, limit: number) {
		var ret = {};
		var count = 0;
		_.each(data, function(v,name) {
			if (limit && count >= limit){
				return
			}
			var s = moment(v.Start).utc();
			var e = moment(v.End).utc();
			if (startBefore && s > startBefore) {
				return;
			}
			if (startAfter && s< startAfter) {
				return;
			}
			if (endAfter && e < endAfter) {
				return;
			}
			if (endBefore && e > endBefore) {
				return;
			}
			ret[name] = v;
		});
		return ret;
	}
	function get() {
		$http.get('/api/silence/get')
			.success((data: any) => {
				$scope.silences = [];
				var now = moment.utc();
				$scope.silences.push({
					name: 'Active',
					silences: filter(data, now, null, now, null, 0)
				});
				$scope.silences.push({
					name: 'Upcoming',
					silences: filter(data, null, now, null, null, 0)
				});
				$scope.silences.push({
					name: 'Past',
					silences: filter(data, null, null, null, now, 25)
				});
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
			message: $scope.message,
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
		$location.search('message', $scope.message || null);
		$route.reload();
	};
	$scope.confirm = () => {
		$scope.error = null;
		$scope.testSilences = null;
		$scope.edit = null;
		$location.search('edit', null);
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
		$http.post('/api/silence/clear?id=' + id, {} )
			.error((error) => {
				$scope.error = error;
			})
			.finally(get);
	};
	$scope.time = (v: any) => {
		var m = moment(v).utc();
		return m.format();
	};
	$scope.getEditSilenceLink = (silence: any, silenceId: string) => {
		return linkService.GetEditSilenceLink(silence, silenceId);
	};
}]);
