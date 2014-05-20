tsafApp.directive('tsRickshaw', ['$filter', '$timeout', function($filter: ng.IFilterService, $timeout: ng.ITimeoutService) {
	return {
		link: (scope: ng.IScope, elem: any, attrs: any) => {
			scope.$watch(attrs.tsRickshaw, function(v: any) {
				$timeout(function() {
					if (!angular.isArray(v) || v.length == 0) {
						return;
					}
					elem[0].innerHTML = '<div class="row"><div class="col-lg-12"><div class="y_axis"></div><div class="rgraph"></div></div></div><div class="row"><div class="col-lg-12"><div class="rlegend"></div></div></div>';
					var palette: any = new Rickshaw.Color.Palette();
					angular.forEach(v, function(i) {
						if (!i.color) {
							i.color = palette.color();
						}
					});
					var rgraph = angular.element('.rgraph', elem);
					var graph_options: any = {
						element: rgraph[0],
						height: rgraph.height(),
						min: 'auto',
						series: v,
						renderer: 'line',
						interpolation: 'linear',
					}
					if (attrs.max) {
						graph_options.max = attrs.max;
					}
					if (attrs.renderer) {
						graph_options.renderer = attrs.renderer;
					}
					var graph: any = new Rickshaw.Graph(graph_options);
					var x_axis: any = new Rickshaw.Graph.Axis.Time({
						graph: graph,
						timeFixture: new Rickshaw.Fixtures.Time(),
					});
					var y_axis: any = new Rickshaw.Graph.Axis.Y({
						graph: graph,
						orientation: 'left',
						tickFormat: function(y: any) {
							var o: any = d3.formatPrefix(y)
						// The precision arg to d3.formatPrefix seems broken, so using round
						// http://stackoverflow.com/questions/10310613/variable-precision-in-d3-format
						return d3.round(o.scale(y), 2) + o.symbol;
						},
						element: angular.element('.y_axis', elem)[0],
					});
					if (attrs.bytes == "true") {
						y_axis.tickFormat = function(y: any) {
							return $filter('bytes')(y);
						}
					}
					graph.render();
					var fmter = 'nfmt';
					if (attrs.bytes == 'true') {
						fmter = 'bytes';
					} else if (attrs.bits == 'true') {
						fmter = 'bits';
					}
					var fmt = $filter(fmter);
					var legend = angular.element('.rlegend', elem)[0];
					var Hover = Rickshaw.Class.create(Rickshaw.Graph.HoverDetail, {
						render: function(args: any) {
							legend.innerHTML = args.formattedXValue;
							args.detail.
								sort((a: any, b: any) => { return a.order - b.order }).
								forEach(function(d: any) {
									var line = document.createElement('div');
									line.className = 'rline';
									var swatch = document.createElement('div');
									swatch.className = 'rswatch';
									swatch.style.backgroundColor = d.series.color;
									var label = document.createElement('div');
									label.className = 'rlabel';
									label.innerHTML = d.name + ": " + fmt(d.formattedYValue);
									line.appendChild(swatch);
									line.appendChild(label);
									legend.appendChild(line);
									var dot = document.createElement('div');
									dot.className = 'dot';
									dot.style.top = graph.y(d.value.y0 + d.value.y) + 'px';
									dot.style.borderColor = d.series.color;
									this.element.appendChild(dot);
									dot.className = 'dot active';
									this.show();
								}, this);
						}
					});
					var hover = new Hover({ graph: graph });
					//Simulate a movemove so the hover appears on load
					var e = document.createEvent('MouseEvents');
					e.initEvent('mousemove', true, false);
					rgraph[0].children[0].dispatchEvent(e);
				});
			});
		},
	};
}]);