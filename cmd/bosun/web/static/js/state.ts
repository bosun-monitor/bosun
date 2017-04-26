
bosunApp.directive('tsAckGroup', ['$location', '$timeout', ($location: ng.ILocationService, $timeout: ng.ITimeoutService) => {
	return {
		scope: {
			ack: '=',
			groups: '=tsAckGroup',
			schedule: '=',
			timeanddate: '=',
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
				
				if (scope.shown[i] && scope.groups[i].Children.length == 1){
					$timeout(function(){
						scope.$broadcast("onOpen", i);
					}, 0);      
				}
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
					if (g.Active && g.Status != 'unknown' && g.Status != 'error') {
						scope.canCloseSelected = false;
					}
					if (g.Status != 'unknown') {
						scope.canForgetSelected = false;
					}
				}
			};
			scope.multiaction = (type: string) => {
				var keys = [];
				angular.forEach(scope.groups, (group) => {
					if (!group.checked) {
						return;
					}
					if (group.AlertKey) {
						keys.push(group.AlertKey);
					}
					angular.forEach(group.Children, (child) => {
						keys.push(child.AlertKey);
					});
				});
				scope.$parent.setKey("action-keys", keys);
				$location.path("action");
				$location.search("type", type);
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
}]);

bosunApp.directive('tsState', ['$sce', '$http', function($sce: ng.ISCEService, $http: ng.IHttpService) {
	return {
		templateUrl: '/partials/alertstate.html',
		link: function(scope: any, elem: any, attrs: any) {
			var myIdx = attrs["tsGrp"];
			scope.currentStatus = attrs["tsGrpstatus"]
			scope.name = scope.child.AlertKey;
			scope.state = scope.child.State;
			scope.action = (type: string) => {
				var key = encodeURIComponent(scope.name);
				return '/action?type=' + type + '&key=' + key;
			};
			var loadedBody = false;
			scope.toggle = () =>{
				scope.show = !scope.show;
				if(scope.show && !loadedBody){
					scope.state.Body = "loading...";
					loadedBody = true;
					$http.get('/api/status?ak='+scope.child.AlertKey)
						.success(data => {
							var body = data[scope.child.AlertKey].Body;
							scope.state.Body = $sce.trustAsHtml(body);
						})
						.error(err => {
							scope.state.Body = "Error loading template body: " + err;
						});
				}
			};
			scope.$on('onOpen', function(e,i) { 
				if(i == myIdx){ 
        			scope.toggle();
				}        
    		});
			scope.zws = (v: string) => {
				if (!v) {
					return '';
				}
				return v.replace(/([,{}()])/g, '$1\u200b');
			};
			scope.state.Touched = moment(scope.state.Touched).utc();
			angular.forEach(scope.state.Events, (v, k) => {
				v.Time = moment(v.Time).utc();
			});
			scope.state.last = scope.state.Events[scope.state.Events.length - 1];
			if (scope.state.Actions && scope.state.Actions.length > 0) {
				scope.state.LastAction = scope.state.Actions[scope.state.Actions.length-1];
			}
			scope.state.RuleUrl = '/config?' +
				'alert=' + encodeURIComponent(scope.state.Alert) +
				'&fromDate=' + encodeURIComponent(scope.state.last.Time.format("YYYY-MM-DD")) +
				'&fromTime=' + encodeURIComponent(scope.state.last.Time.format("HH:mm"));
			var groups: string[] = [];
			angular.forEach(scope.state.Group, (v, k) => {
				groups.push(k + "=" + v);
			});
			if (groups.length > 0) {
				scope.state.RuleUrl += '&template_group=' + encodeURIComponent(groups.join(','));
			}
			scope.state.Body = $sce.trustAsHtml(scope.state.Body);
		},
	};
}]);

bosunApp.directive('tsNote', () => {
	return {
		restrict: 'E',
		templateUrl: '/partials/note.html',
	};
});


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

bosunApp.directive('tsDelayedClose', () => {
	return {
		restrict: 'E',
		templateUrl: '/partials/delayedClose.html',
	};
});

bosunApp.directive('tsForget', () => {
	return {
		restrict: 'E',
		templateUrl: '/partials/forget.html',
	};
});

bosunApp.directive('tsPurge', () => {
	return {
		restrict: 'E',
		templateUrl: '/partials/purge.html',
	};
});

bosunApp.directive('tsForceClose', () => {
	return {
		restrict: 'E',
		templateUrl: '/partials/forceClose.html',
	};
});
