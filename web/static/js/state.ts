tsafApp.directive('tsAckGroup', function() {
	return {
		scope: {
			ack: '=',
			groups: '=tsAckGroup',
			schedule: '=schedule',
		},
		templateUrl: '/partials/ackgroup.html',
		link: (scope: any, elem: any, attrs: any) => {
			scope.canAckSelected = scope.ack == 'Needs Acknowldgement';
			scope.panelClass = (status: string) => {
				switch (status) {
					case "critical": return "panel-danger";
					case "unknown": return "panel-info";
					case "warning": return "panel-warning";
					default: return "panel-default";
				}
			};
			scope.btoa = scope.$parent.btoa;
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
				scope.canCloseSelected = scope.canForgetSelected = true;
				scope.anySelected = false;
				for (var i = 0; i < scope.groups.length; i++) {
					var g = scope.groups[i];
					if (!g.checked) {
						continue;
					}
					scope.anySelected = true;
					if (g.Active) {
						scope.canCloseSelected = false;
						scope.canForgetSelected = false;
					}
					if (g.Status != "unknown") {
						scope.canForgetSelected = false;
					}
				}
			};
			scope.multiaction = (type: string) => {
				var url = '/action?type=' + type;
				angular.forEach(scope.groups, (group) => {
					if (group.checked) {
						url += '&key=' + encodeURIComponent(group.AlertKey);
					}
				});
				return url;
			};
		},
	};
});

tsafApp.directive('tsState', function() {
	return {
		templateUrl: '/partials/alertstate.html',
		link: function(scope: any, elem: any, attrs: any) {
			scope.action = (type: string) => {
				var key = encodeURIComponent(scope.name);
				return '/action?type=' + type + '&key=' + key;
			};
			scope.zws = (v: string) => {
				return v.replace(/([,{}()])/g, '$1\u200b');
			};
		},
	};
});

tsafApp.directive('tsAck', () => {
	return {
		restrict: 'E',
		templateUrl: '/partials/ack.html',
	};
});

tsafApp.directive('tsClose', () => {
	return {
		restrict: 'E',
		templateUrl: '/partials/close.html',
	};
});

tsafApp.directive('tsForget', () => {
	return {
		restrict: 'E',
		templateUrl: '/partials/forget.html',
	};
});