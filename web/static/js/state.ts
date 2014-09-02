bosunApp.directive('tsAckGroup', function() {
	return {
		scope: {
			ack: '=',
			groups: '=tsAckGroup',
			schedule: '=',
		},
		templateUrl: '/partials/ackgroup.html',
		link: (scope: any, elem: any, attrs: any) => {
			scope.canAckSelected = scope.ack == 'Needs Acknowledgement';
			scope.panelClass = scope.$parent.panelClass;
			scope.btoa = scope.$parent.btoa;
			scope.encode = scope.$parent.encode;
			scope.shown = {};
			scope.collapse = (i: any) => {
				scope.shown[i] = !scope.shown[i];
			};
			scope.click = ($event: any, idx: number) => {
				scope.collapse(idx);
				if ($event.shiftKey && scope.schedule.checkIdx != undefined) {
					var checked = scope.groups[scope.schedule.checkIdx].checked;
					var start = Math.min(idx, scope.schedule.checkIdx);
					var end = Math.max(idx, scope.schedule.checkIdx);
					for (var i = start; i <= end; i++) {
						if (i == idx) {
							continue;
						}
						scope.groups[i].checked = checked;
					}
				}
				scope.schedule.checkIdx = idx;
				scope.update();
			};
			scope.select = (checked: boolean) => {
				for (var i = 0; i < scope.groups.length; i++) {
					scope.groups[i].checked = checked;
				}
				scope.update();
			};
			scope.update = () => {
				scope.canCloseSelected = true;
				scope.canForgetSelected = true;
				scope.anySelected = false;
				for (var i = 0; i < scope.groups.length; i++) {
					var g = scope.groups[i];
					if (!g.checked) {
						continue;
					}
					scope.anySelected = true;
					if (g.Active && g.Status != 'unknown') {
						scope.canCloseSelected = false;
					}
					if (g.Status != 'unknown') {
						scope.canForgetSelected = false;
					}
				}
			};
			scope.multiaction = (type: string) => {
				var url = '/action?type=' + type;
				angular.forEach(scope.groups, (group) => {
					if (!group.checked) {
						return;
					}
					if (group.AlertKey) {
						url += '&key=' + encodeURIComponent(group.AlertKey);
					}
					angular.forEach(group.Children, (child) => {
						url += '&key=' + encodeURIComponent(child.AlertKey);
					});
				});
				return url;
			};
			scope.history = () => {
				var url = '/history?';
				angular.forEach(scope.groups, (group) => {
					if (!group.checked) {
						return;
					}
					if (group.AlertKey) {
						url += '&key=' + encodeURIComponent(group.AlertKey);
					}
					angular.forEach(group.Children, (child) => {
						url += '&key=' + encodeURIComponent(child.AlertKey);
					});
				});
				return url;
			};
		},
	};
});

bosunApp.factory('status', ['$http', '$q', function($http: ng.IHttpService, $q: ng.IQService) {
	var cache: any = {};
	return function(ak: string) {
		var q = $q.defer();
		if (cache[ak]) {
			q.resolve(cache[ak]);
		} else {
			$http.get('/api/status?ak=' + encodeURIComponent(ak))
				.success(data => {
					angular.forEach(data, (v, k) => {
						v.Touched = moment(v.Touched).utc();
						angular.forEach(v.History, (v, k) => {
							v.Time = moment(v.Time).utc();
						});
						v.last = v.History[v.History.length - 1];
						cache[k] = v;
					});
					q.resolve(cache[ak]);
				})
				.error(q.reject);
		}
		return q.promise;
	};
}]);

bosunApp.directive('tsState', ['status', function($status: any) {
	return {
		templateUrl: '/partials/alertstate.html',
		link: function(scope: any, elem: any, attrs: any) {
			scope.name = scope.child.AlertKey;
			scope.loading = true;
			$status(scope.child.AlertKey).then(st => {
					scope.state = st;
					scope.loading = false;
				}, err => {
					alert(err);
					scope.loading = false;
				});
			scope.action = (type: string) => {
				var key = encodeURIComponent(scope.name);
				return '/action?type=' + type + '&key=' + key;
			};
			scope.zws = (v: string) => {
				if (!v) {
					return '';
				}
				return v.replace(/([,{}()])/g, '$1\u200b');
			};
		},
	};
}]);

bosunApp.directive('tsAck', () => {
	return {
		restrict: 'E',
		templateUrl: '/partials/ack.html',
	};
});

bosunApp.directive('tsClose', () => {
	return {
		restrict: 'E',
		templateUrl: '/partials/close.html',
	};
});

bosunApp.directive('tsForget', () => {
	return {
		restrict: 'E',
		templateUrl: '/partials/forget.html',
	};
});
