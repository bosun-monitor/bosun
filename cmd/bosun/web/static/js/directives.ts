/// <reference path="0-bosun.ts" />

bosunApp.directive('tsResults', function() {
    return {
        templateUrl: '/partials/results.html',
        link: (scope: any, elem, attrs) => {
            scope.isSeries = v => {
                return typeof (v) === 'object';
            };
        },
    };
});

bosunApp.directive('tsComputations', () => {
    return {
        scope: {
            computations: '=tsComputations',
            time: '=',
            header: '=',
        },
        templateUrl: '/partials/computations.html',
        link: (scope: any, elem: any, attrs: any) => {
            if (scope.time) {
                var m = moment.utc(scope.time);
                scope.timeParam = "&date=" + encodeURIComponent(m.format("YYYY-MM-DD")) + "&time=" + encodeURIComponent(m.format("HH:mm"));
            }
            scope.btoa = (v: any) => {
                return encodeURIComponent(btoa(v));
            };
        },
    };
});


function fmtDuration(v: any) {
    var diff = (moment.duration(v, 'milliseconds'));
    var f;
    if (Math.abs(v) < 60000) {
        return diff.format('ss[s]');
    }
    return diff.format('d[d]hh[h]mm[m]ss[s]');
}


function fmtTime(v: any) {
    var m = moment(v).utc();
    var now = moment().utc();
    var msdiff = now.diff(m);
    var ago = '';
    var inn = '';
    if (msdiff >= 0) {
        ago = ' ago';
    } else {
        inn = 'in ';
    }
    return m.format() + ' UTC (' + inn + fmtDuration(Math.abs(msdiff)) + ago + ')';
}

function parseDuration(v: string) {
    var pattern = /(\d+)(d|y|n|h|m|s)(-ago)?/;
    var m = pattern.exec(v);
    if (m) {
        return moment.duration(parseInt(m[1]), m[2].replace('n', 'M'))
    }
    return moment.duration(0)
}

interface ITimeScope extends IBosunScope {
    noLink: string;
}

bosunApp.directive("tsTime", function() {
    return {
        link: function(scope: ITimeScope, elem: any, attrs: any) {
            scope.$watch(attrs.tsTime, (v: any) => {
                var m = moment(v).utc();
                var text = fmtTime(v);
                if (attrs.tsEndTime) {
                    var diff = moment(scope.$eval(attrs.tsEndTime)).diff(m);
                    var duration = fmtDuration(diff);
                    text += " for " + duration;
                }
                if (attrs.noLink) {
                    elem.text(text);
                } else {
                    var el = document.createElement('a');
                    el.text = text;
                    el.href = 'http://www.timeanddate.com/worldclock/converted.html?iso=';
                    el.href += m.format('YYYYMMDDTHHmm');
                    el.href += '&p1=0';
                    angular.forEach(scope.timeanddate, (v, k) => {
                        el.href += '&p' + (k + 2) + '=' + v;
                    });
                    elem.html(el);
                }
            });
        },
    };
});

bosunApp.directive("tsTimeUnix", function() {
    return {
        link: function(scope: ITimeScope, elem: any, attrs: any) {
            scope.$watch(attrs.tsTimeUnix, (v: any) => {
                var m = moment(v * 1000).utc();
                var text = fmtTime(m);
                if (attrs.tsEndTime) {
                    var diff = moment(scope.$eval(attrs.tsEndTime)).diff(m);
                    var duration = fmtDuration(diff);
                    text += " for " + duration;
                }
                if (attrs.noLink) {
                    elem.text(text);
                } else {
                    var el = document.createElement('a');
                    el.text = text;
                    el.href = 'http://www.timeanddate.com/worldclock/converted.html?iso=';
                    el.href += m.format('YYYYMMDDTHHmm');
                    el.href += '&p1=0';
                    angular.forEach(scope.timeanddate, (v, k) => {
                        el.href += '&p' + (k + 2) + '=' + v;
                    });
                    elem.html(el);
                }
            });
        },
    };
});

bosunApp.directive("tsSince", function() {
    return {
        link: function(scope: IBosunScope, elem: any, attrs: any) {
            scope.$watch(attrs.tsSince, (v: any) => {
                var m = moment(v).utc();
                elem.text(m.fromNow());
            });
        },
    };
});

bosunApp.directive("tooltip", function() {
    return {
        link: function(scope: IGraphScope, elem: any, attrs: any) {
            angular.element(elem[0]).tooltip({ placement: "bottom" });
        },
    };
});


bosunApp.directive('tsTab', () => {
    return {
        link: (scope: any, elem: any, attrs: any) => {
            var ta = elem[0];
            elem.keydown(evt => {
                if (evt.ctrlKey) {
                    return;
                }
                // This is so shift-enter can be caught to run a rule when tsTab is called from
                // the rule page
                if (evt.keyCode == 13 && evt.shiftKey) {
                    return;
                }
                switch (evt.keyCode) {
                    case 9: // tab
                        evt.preventDefault();
                        var v = ta.value;
                        var start = ta.selectionStart;
                        ta.value = v.substr(0, start) + "\t" + v.substr(start);
                        ta.selectionStart = ta.selectionEnd = start + 1;
                        return;
                    case 13: // enter
                        if (ta.selectionStart != ta.selectionEnd) {
                            return;
                        }
                        evt.preventDefault();
                        var v = ta.value;
                        var start = ta.selectionStart;
                        var sub = v.substr(0, start);
                        var last = sub.lastIndexOf("\n") + 1
                        for (var i = last; i < sub.length && /[ \t]/.test(sub[i]); i++)
                            ;
                        var ws = sub.substr(last, i - last);
                        ta.value = v.substr(0, start) + "\n" + ws + v.substr(start);
                        ta.selectionStart = ta.selectionEnd = start + 1 + ws.length;
                }
            });
        },
    };
});

interface JQuery {
    tablesorter(v: any): JQuery;
    linedtextarea(): void;
}

bosunApp.directive('tsresizable', () => {
    return {
        restrict: 'A',
        scope: {
            callback: '&onResize'
        },
        link: function postLink(scope: any, elem: any, attrs) {
            elem.resizable();
            elem.on('resizestop', function(evt, ui) {
                if (scope.callback) { scope.callback(); }
            });
        }
    };
});

bosunApp.directive('tsTableSort', ['$timeout', ($timeout: ng.ITimeoutService) => {
    return {
        link: (scope: ng.IScope, elem: any, attrs: any) => {
            $timeout(() => {
                $(elem).tablesorter({
                    sortList: scope.$eval(attrs.tsTableSort),
                });
            });
        },
    };
}]);

// https://gist.github.com/mlynch/dd407b93ed288d499778
bosunApp.directive('autofocus', ['$timeout', function($timeout) {
  return {
    restrict: 'A',
    link : function($scope, $element) {
      $timeout(function() {
        $element[0].focus();
      });
    }
  }
}]);

bosunApp.directive('tsTimeLine', () => {
    var tsdbFormat = d3.time.format.utc("%Y/%m/%d-%X");
    function parseDate(s: any) {
        return moment.utc(s).toDate();
    }
    var margin = {
        top: 10,
        right: 10,
        bottom: 30,
        left: 250,
    };
    return {
        link: (scope: any, elem: any, attrs: any) => {
            scope.shown = {};
            scope.collapse = (i: any, entry: any, v: any) => {
                scope.shown[i] = !scope.shown[i];
                if (scope.loadTimelinePanel && entry && scope.shown[i]) {
                    scope.loadTimelinePanel(entry, v);
                }
            };
            scope.$watch('alert_history', update);
            function update(history: any) {
                if (!history) {
                    return;
                }
                var entries = d3.entries(history);
                if (!entries.length) {
                    return;
                }
                entries.sort((a, b) => {
                    return a.key.localeCompare(b.key);
                });
                scope.entries = entries;
                var values = entries.map(v => { return v.value });
                var keys = entries.map(v => { return v.key });
                var barheight = 500 / values.length;
                barheight = Math.min(barheight, 45);
                barheight = Math.max(barheight, 15);
                var svgHeight = values.length * barheight + margin.top + margin.bottom;
                var height = svgHeight - margin.top - margin.bottom;
                var svgWidth = elem.width();
                var width = svgWidth - margin.left - margin.right;
                var xScale = d3.time.scale.utc().range([0, width]);
                var xAxis = d3.svg.axis()
                    .scale(xScale)
                    .orient('bottom');
                elem.empty();
                var svg = d3.select(elem[0])
                    .append('svg')
                    .attr('width', svgWidth)
                    .attr('height', svgHeight)
                    .append('g')
                    .attr('transform', 'translate(' + margin.left + ',' + margin.top + ')');
                svg.append('g')
                    .attr('class', 'x axis tl-axis')
                    .attr('transform', 'translate(0,' + height + ')');
                xScale.domain([
                    d3.min(values, (d: any) => { return d3.min(d.History, (c: any) => { return parseDate(c.Time); }); }),
                    d3.max(values, (d: any) => { return d3.max(d.History, (c: any) => { return parseDate(c.EndTime); }); }),
                ]);
                var legend = d3.select(elem[0])
                    .append('div')
                    .attr('class', 'tl-legend');
                var time_legend = legend
                    .append('div')
                    .text(values[0].History[0].Time);
                var alert_legend = legend
                    .append('div')
                    .text(keys[0]);
                svg.select('.x.axis')
                    .transition()
                    .call(xAxis);
                var chart = svg.append('g');
                angular.forEach(entries, function(entry: any, i: number) {
                    chart.selectAll('.bars')
                        .data(entry.value.History)
                        .enter()
                        .append('rect')
                        .attr('class', (d: any) => { return 'tl-' + d.Status; })
                        .attr('x', (d: any) => { return xScale(parseDate(d.Time)); })
                        .attr('y', i * barheight)
                        .attr('height', barheight)
                        .attr('width', (d: any) => {
                            return xScale(parseDate(d.EndTime)) - xScale(parseDate(d.Time));
                        })
                        .on('mousemove.x', mousemove_x)
                        .on('mousemove.y', function(d) {
                            alert_legend.text(entry.key);
                        })
                        .on('click', function(d, j) {
                            var id = 'panel' + i + '-' + j;
                            scope.shown['group' + i] = true;
                            scope.shown[id] = true;
                            if (scope.loadTimelinePanel) {
                                scope.loadTimelinePanel(entry, d);
                            }

                            scope.$apply();
                            setTimeout(() => {
                                var e = $("#" + id);
                                if (!e) {
                                    console.log('no', id, e);
                                    return;
                                }
                                $('html, body').scrollTop(e.offset().top);
                            });
                        });
                });
                chart.selectAll('.labels')
                    .data(keys)
                    .enter()
                    .append('text')
                    .attr('text-anchor', 'end')
                    .attr('x', 0)
                    .attr('dx', '-.5em')
                    .attr('dy', '.25em')
                    .attr('y', function(d: any, i: number) { return (i + .5) * barheight; })
                    .text(function(d: any) { return d; });
                chart.selectAll('.sep')
                    .data(values)
                    .enter()
                    .append('rect')
                    .attr('y', function(d: any, i: number) { return (i + 1) * barheight })
                    .attr('height', 1)
                    .attr('x', 0)
                    .attr('width', width)
                    .on('mousemove.x', mousemove_x);
                function mousemove_x() {
                    var x = xScale.invert(d3.mouse(this)[0]);
                    time_legend
                        .text(tsdbFormat(x));
                }
            };
        },
    };
});

var fmtUnits = ['', 'k', 'M', 'G', 'T', 'P', 'E'];

function nfmt(s: any, mult: number, suffix: string, opts: any) {
    opts = opts || {};
    var n = parseFloat(s);
    if (isNaN(n) && typeof s === 'string') {
        return s;
    }
    if (opts.round) n = Math.round(n);
    if (!n) return suffix ? '0 ' + suffix : '0';
    if (isNaN(n) || !isFinite(n)) return '-';
    var a = Math.abs(n);
    if (a >= 1) {
        var number = Math.floor(Math.log(a) / Math.log(mult));
        a /= Math.pow(mult, Math.floor(number));
        if (fmtUnits[number]) {
            suffix = fmtUnits[number] + suffix;
        }
    }
    var r = a.toFixed(5);
    if (a < 1e-5) {
        r = a.toString();
    }
    var neg = n < 0 ? '-' : '';
    return neg + (+r) + suffix;
}

bosunApp.filter('nfmt', function() {
    return function(s: any) {
        return nfmt(s, 1000, '', {});
    }
});

bosunApp.filter('bytes', function() {
    return function(s: any) {
        return nfmt(s, 1024, 'B', { round: true });
    }
});

bosunApp.filter('bits', function() {
    return function(s: any) {
        return nfmt(s, 1024, 'b', { round: true });
    }
});


bosunApp.directive('elastic', [
    '$timeout',
    function($timeout) {
        return {
            restrict: 'A',
            link: function($scope, element) {
                $scope.initialHeight = $scope.initialHeight || element[0].style.height;
                var resize = function() {
                    element[0].style.height = $scope.initialHeight;
                    element[0].style.height = "" + element[0].scrollHeight + "px";
                };
                element.on("input change", resize);
                $timeout(resize, 0);
            }
        };
    }
]);

bosunApp.directive('tsBar', ['$window', 'nfmtFilter', function($window: ng.IWindowService, fmtfilter: any) {
    var margin = {
        top: 20,
        right: 20,
        bottom: 0,
        left: 200,
    };
    return {
        scope: {
            data: '=',
            height: '=',
        },
        link: (scope: any, elem: any, attrs: any) => {
            var svgHeight = +scope.height || 150;
            var height = svgHeight - margin.top - margin.bottom;
            var svgWidth: number;
            var width: number;
            var xScale = d3.scale.linear();
            var yScale = d3.scale.ordinal()
            var top = d3.select(elem[0])
                .append('svg')
                .attr('height', svgHeight)
                .attr('width', '100%');
            var svg = top
                .append('g')
            //.attr('transform', 'translate(' + margin.left + ',' + margin.top + ')');
            var xAxis = d3.svg.axis()
                .scale(xScale)
                .orient("top")
            var yAxis = d3.svg.axis()
                .scale(yScale)
                .orient("left")
            scope.$watch('data', update);
            var w = angular.element($window);
            scope.$watch(() => {
                return w.width();
            }, resize, true);
            w.bind('resize', () => {
                scope.$apply();
            });
            function resize() {
                if (!scope.data) {
                    return;
                }
                svgWidth = elem.width();
                if (svgWidth <= 0) {
                    return;
                }
                margin.left = d3.max(scope.data, (d: any) => { return d.name.length * 8 })
                width = svgWidth - margin.left - margin.right;
                svgHeight = scope.data.length * 15;
                height = svgHeight - margin.top - margin.bottom;
                xScale.range([0, width]);
                yScale.rangeRoundBands([0, height], .1);
                yAxis.scale(yScale);
                svg.attr('transform', 'translate(' + margin.left + ',' + margin.top + ')');
                svg.attr('width', svgWidth);
                svg.attr('height', height);
                top.attr('height', svgHeight);
                xAxis.ticks(width / 60);
                draw();
            }
            function update(v: any) {
                if (!angular.isArray(v) || v.length == 0) {
                    return;
                }
                resize();
            }
            function draw() {
                if (!scope.data) {
                    return;
                }
                yScale.domain(scope.data.map((d: any) => { return d.name }));
                xScale.domain([0, d3.max(scope.data, (d: any) => { return d.Value })]);
                svg.selectAll('g.axis').remove();
                //X axis
                svg.append("g")
                    .attr("class", "x axis")
                    .call(xAxis)
                svg.append("g")
                    .attr("class", "y axis")
                    .call(yAxis)
                    .selectAll("text")
                    .style("text-anchor", "end")
                var bars = svg.selectAll(".bar").data(scope.data);
                bars.enter()
                    .append("rect")
                    .attr("class", "bar")
                    .attr("y", function(d) { return yScale(d.name); })
                    .attr("height", yScale.rangeBand())
                    .attr('width', (d: any) => { return xScale(d.Value); })
            };
        },
    };
}]);

bosunApp.directive('tsGraph', ['$window', 'nfmtFilter', function($window: ng.IWindowService, fmtfilter: any) {
    var margin = {
        top: 10,
        right: 10,
        bottom: 30,
        left: 80,
    };
    return {
        scope: {
            data: '=',
            annotations: '=',
            height: '=',
            generator: '=',
            brushStart: '=bstart',
            brushEnd: '=bend',
            enableBrush: '@',
            max: '=',
            min: '=',
            normalize: '=',
            annotation: '=',
            annotateEnabled: '=',
            showAnnotations: '=',
        },
        template: '<div class="row"></div>' + // chartElemt
        '<div class="row col-lg-12"></div>' + // timeElem
        '<div class"row">' + // legendAnnContainer
        '<div class="col-lg-6"></div>' + // legendElem
        '<div class="col-lg-6"></div>' + // annElem
        '</div>',
        link: (scope: any, elem: any, attrs: any, $compile: any) => {
            var chartElem = d3.select(elem.children()[0]);
            var timeElem = d3.select(elem.children()[1]);
            var legendAnnContainer = angular.element(elem.children()[2]);
            var legendElem = d3.select(legendAnnContainer.children()[0]);
            if (scope.annotateEnabled) {
                var annElem = d3.select(legendAnnContainer.children()[1]);
            }
            var valueIdx = 1;
            if (scope.normalize) {
                valueIdx = 2;
            }
            var svgHeight = +scope.height || 150;
            var height = svgHeight - margin.top - margin.bottom;
            var svgWidth: number;
            var width: number;
            var yScale = d3.scale.linear().range([height, 0]);
            var xScale = d3.time.scale.utc();
            var xAxis = d3.svg.axis()
                .orient('bottom');
            var yAxis = d3.svg.axis()
                .scale(yScale)
                .orient('left')
                .ticks(Math.min(10, height / 20))
                .tickFormat(fmtfilter);
            var line: any;
            switch (scope.generator) {
                case 'area':
                    line = d3.svg.area();
                    break;
                default:
                    line = d3.svg.line();
            }
            var brush = d3.svg.brush()
                .x(xScale)
                .on('brush', brushed)
                .on('brushend', annotateBrushed);
            var top = chartElem
                .append('svg')
                .attr('height', svgHeight)
                .attr('width', '100%');
            var svg = top
                .append('g')
                .attr('transform', 'translate(' + margin.left + ',' + margin.top + ')');
            var defs = svg.append('defs')
                .append('clipPath')
                .attr('id', 'clip')
                .append('rect')
                .attr('height', height);
            var chart = svg.append('g')
                .attr('pointer-events', 'all')
                .attr('clip-path', 'url(#clip)');
            svg.append('g')
                .attr('class', 'x axis')
                .attr('transform', 'translate(0,' + height + ')');
            svg.append('g')
                .attr('class', 'y axis');
            var paths = chart.append('g');
            chart.append('g')
                .attr('class', 'x brush');
            if (scope.annotateEnabled) {
                var ann = chart.append('g');
            }
            top.append('rect')
                .style('opacity', 0)
                .attr('x', 0)
                .attr('y', 0)
                .attr('height', height)
                .attr('width', margin.left)
                .style('cursor', 'pointer')
                .on('click', yaxisToggle);
            var xloc = timeElem.append('div').attr("class", "col-lg-6");
            xloc.style('float', 'left');
            var brushText = timeElem.append('div').attr("class", "col-lg-6").append('p').attr("class", "text-right")
            var legend = legendElem;
            var aLegend = annElem;

            var color = d3.scale.ordinal().range([
                '#e41a1c',
                '#377eb8',
                '#4daf4a',
                '#984ea3',
                '#ff7f00',
                '#a65628',
                '#f781bf',
                '#999999',
            ]);
            var annColor = d3.scale.ordinal().range([
                '#e41a1c',
                '#377eb8',
                '#4daf4a',
                '#984ea3',
                '#ff7f00',
                '#a65628',
                '#f781bf',
                '#999999',
            ]);
            var mousex = 0;
            var mousey = 0;
            var oldx = 0;
            var hover = svg.append('g')
                .attr('class', 'hover')
                .style('pointer-events', 'none')
                .style('display', 'none');
            var hoverPoint = hover.append('svg:circle')
                .attr('r', 5);
            var hoverRect = hover.append('svg:rect')
                .attr('fill', 'white');
            var hoverText = hover.append('svg:text')
                .style('font-size', '12px');
            var focus = svg.append('g')
                .attr('class', 'focus')
                .style('pointer-events', 'none');
            focus.append('line');
            var yaxisZero = false;
            function yaxisToggle() {
                yaxisZero = !yaxisZero;
                draw();
            }

            var drawAnnLegend = () => {
                if (scope.annotation) {
                    aLegend.html('')
                    var a = scope.annotation;
                    //var table = aLegend.append('table').attr("class", "table table-condensed")
                    var table = aLegend.append("div")
                    var row = table.append("div").attr("class", "row")
                    row.append("div").attr("class", "col-lg-2").text("CreationUser")
                    row.append("div").attr("class", "col-lg-10").text(a.CreationUser)
                    row = table.append("div").attr("class", "row")
                    row.append("div").attr("class", "col-lg-2").text("Owner")
                    row.append("div").attr("class", "col-lg-10").text(a.Owner)
                    row = table.append("div").attr("class", "row")
                    row.append("div").attr("class", "col-lg-2").text("Url")
                    row.append("div").attr("class", "col-lg-10").append('a')
                        .attr("xlink:href", a.Url).text(a.Url).on("click", (d) => {
                            window.open(a.Url, "_blank");
                        });
                    row = table.append("div").attr("class", "row")
                    row.append("div").attr("class", "col-lg-2").text("Category")
                    row.append("div").attr("class", "col-lg-10").text(a.Category)
                    row = table.append("div").attr("class", "row")
                    row.append("div").attr("class", "col-lg-2").text("Host")
                    row.append("div").attr("class", "col-lg-10").text(a.Host)
                    row = table.append("div").attr("class", "row")
                    row.append("div").attr("class", "col-lg-2").text("Message")
                    row.append("div").attr("class", "col-lg-10").text(a.Message)
                }//
            };

            var drawLegend = _.throttle((normalizeIdx: any) => {
                var names = legend.selectAll('.series')
                    .data(scope.data, (d) => { return d.Name; });
                names.enter()
                    .append('div')
                    .attr('class', 'series');
                names.exit()
                    .remove();


                var xi = xScale.invert(mousex);
                xloc.text('Time: ' + fmtTime(xi));
                var t = xi.getTime() / 1000;
                var minDist = width + height;
                var minName: string, minColor: string;
                var minX: number, minY: number;

                names
                    .each(function(d: any) {
                        var idx = bisect(d.Data, t);
                        if (idx >= d.Data.length) {
                            idx = d.Data.length - 1;
                        }
                        var e = d3.select(this);
                        var pt = d.Data[idx];
                        if (pt) {
                            e.attr('title', pt[normalizeIdx]);
                            e.text(d.Name + ': ' + fmtfilter(pt[1]));
                            var ptx = xScale(pt[0] * 1000);
                            var pty = yScale(pt[normalizeIdx]);
                            var ptd = Math.sqrt(
                                Math.pow(ptx - mousex, 2) +
                                Math.pow(pty - mousey, 2)
                            );
                            if (ptd < minDist) {
                                minDist = ptd;
                                minX = ptx;
                                minY = pty;
                                minName = d.Name + ': ' + pt[1];
                                minColor = color(d.Name);
                            }
                        }
                    })
                    .style('color', (d: any) => { return color(d.Name); });
                hover
                    .attr('transform', 'translate(' + minX + ',' + minY + ')');
                hoverPoint.style('fill', minColor);
                hoverText
                    .text(minName)
                    .style('fill', minColor);
                var isRight = minX > width / 2;
                var isBottom = minY > height / 2;
                hoverText
                    .attr('x', isRight ? -5 : 5)
                    .attr('y', isBottom ? -8 : 15)
                    .attr('text-anchor', isRight ? 'end' : 'start');
                var node: any = hoverText.node();
                var bb = node.getBBox();
                hoverRect
                    .attr('x', bb.x - 1)
                    .attr('y', bb.y - 1)
                    .attr('height', bb.height + 2)
                    .attr('width', bb.width + 2);
                var x = mousex;
                if (x > width) {
                    x = 0;
                }
                focus.select('line')
                    .attr('x1', x)
                    .attr('x2', x)
                    .attr('y1', 0)
                    .attr('y2', height);
                if (extentStart) {
                    var s = extentStart;
                    if (extentEnd != extentStart) {
                        s += ' - ' + extentEnd;
                        s += ' (' + extentDiff + ')'
                    }
                    brushText.text(s);
                }
            }, 50);

            scope.$watchCollection('[data, annotations, showAnnotations]', update);
            var showAnnotations = (show: boolean) => {
                if (show) {
                    ann.attr("visibility", "visible");
                    return;
                }
                ann.attr("visibility", "hidden");
                aLegend.html('');
            }
            var w = angular.element($window);
            scope.$watch(() => {
                return w.width();
            }, resize, true);
            w.bind('resize', () => {
                scope.$apply();
            });
            function resize() {
                svgWidth = elem.width();
                if (svgWidth <= 0) {
                    return;
                }
                width = svgWidth - margin.left - margin.right;
                xScale.range([0, width]);
                xAxis.scale(xScale);
                if (!mousex) {
                    mousex = width + 1;
                }
                svg.attr('width', svgWidth);
                defs.attr('width', width);
                xAxis.ticks(width / 60);
                draw();
            }
            var oldx = 0;
            var bisect = d3.bisector((d) => { return d[0]; }).left;
            var bisectA = d3.bisector((d) => { return moment(d.StartDate).unix(); }).left;
            function update(v: any) {
                if (!angular.isArray(v) || v.length == 0) {
                    return;
                }
                d3.selectAll(".x.brush").call(brush.clear());
                if (scope.annotateEnabled) {
                    showAnnotations(scope.showAnnotations);
                }
                resize();
            }
            function draw() {
                if (!scope.data) {
                    return;
                }
                if (scope.normalize) {
                    valueIdx = 2;
                }
                function mousemove() {
                    var pt = d3.mouse(this);
                    mousex = pt[0];
                    mousey = pt[1];
                    drawLegend(valueIdx);
                }
                scope.data.map((data: any, i: any) => {
                    var max = d3.max(data.Data, (d: any) => { return d[1]; });
                    data.Data.map((d: any, j: any) => {
                        d.push(d[1] / max * 100 || 0)
                    });
                });
                line.y((d: any) => { return yScale(d[valueIdx]); });
                line.x((d: any) => { return xScale(d[0] * 1000); });
                var xdomain = [
                    d3.min(scope.data, (d: any) => { return d3.min(d.Data, (c: any) => { return c[0]; }); }) * 1000,
                    d3.max(scope.data, (d: any) => { return d3.max(d.Data, (c: any) => { return c[0]; }); }) * 1000,
                ];
                if (!oldx) {
                    oldx = xdomain[1];
                }
                xScale.domain(xdomain);
                var ymin = d3.min(scope.data, (d: any) => { return d3.min(d.Data, (c: any) => { return c[1]; }); });
                var ymax = d3.max(scope.data, (d: any) => { return d3.max(d.Data, (c: any) => { return c[valueIdx]; }); });
                var diff = (ymax - ymin) / 50;
                if (!diff) {
                    diff = 1;
                }
                ymin -= diff;
                ymax += diff;
                if (yaxisZero) {
                    if (ymin > 0) {
                        ymin = 0;
                    } else if (ymax < 0) {
                        ymax = 0;
                    }
                }
                var ydomain = [ymin, ymax];
                if (angular.isNumber(scope.min)) {
                    ydomain[0] = +scope.min;
                }
                if (angular.isNumber(scope.max)) {
                    ydomain[valueIdx] = +scope.max;
                }
                yScale.domain(ydomain);
                if (scope.generator == 'area') {
                    line.y0(yScale(0));
                }
                svg.select('.x.axis')
                    .transition()
                    .call(xAxis);
                svg.select('.y.axis')
                    .transition()
                    .call(yAxis);
                svg.append('text')
                    .attr("class", "ylabel")
                    .attr("transform", "rotate(-90)")
                    .attr("y", -margin.left)
                    .attr("x", - (height / 2))
                    .attr("dy", "1em")
                    .text(_.uniq(scope.data.map(v => { return v.Unit })).join("; "));

                if (scope.annotateEnabled) {
                    var rowId = {}; // annotation Id -> rowId
                    var rowEndDate = {}; // rowId -> EndDate
                    var maxRow = 0;
                    for (var i = 0; i < scope.annotations.length; i++) {
                        if (i == 0) {
                            rowId[scope.annotations[i].Id] = 0;
                            rowEndDate[0] = scope.annotations[0].EndDate;
                            continue;
                        }
                        for (var row = 0; row <= maxRow + 1; row++) {
                            if (row == maxRow + 1) {
                                rowId[scope.annotations[i].Id] = row;
                                rowEndDate[row] = scope.annotations[i].EndDate;
                                maxRow += 1
                                break;
                            }
                            if (rowEndDate[row] < scope.annotations[i].StartDate) {
                                rowId[scope.annotations[i].Id] = row;
                                rowEndDate[row] = scope.annotations[i].EndDate;
                                break;
                            }
                        }
                    }
                    var annotations = ann.selectAll('.annotation')
                        .data(scope.annotations, (d) => { return d.Id; });
                    annotations.enter()
                        .append("svg:a")
                        .append('rect')
                        .attr('visilibity', () => {
                            if (scope.showAnnotations) {
                                return "visible";
                            }
                            return "hidden";
                        })
                        .attr("y", (d) => { return rowId[d.Id] * ((height * .05) + 2) })
                        .attr("height", height * .05)
                        .attr("class", "annotation")
                        .attr("stroke", (d) => { return annColor(d.Id) })
                        .attr("stroke-opacity", .5)
                        .attr("fill", (d) => { return annColor(d.Id) })
                        .attr("fill-opacity", 0.1)
                        .attr("stroke-width", 1)
                        .attr("x", (d: any) => { return xScale(moment(d.StartDate).utc().unix() * 1000); })
                        .attr("width", (d: any) => {
                            var startT = moment(d.StartDate).utc().unix() * 1000
                            var endT = moment(d.EndDate).utc().unix() * 1000
                            var calcWidth = xScale(endT) - xScale(startT)
                            // Never render boxes with less than 8 pixels are they are difficult to click
                            if (calcWidth < 8) {
                                return 8;
                            }
                            return calcWidth;
                        })
                        .on("mouseenter", (ann) => {
                            if (!scope.showAnnotations) {
                                return;
                            }
                            if (ann) {
                                scope.annotation = ann;
                                drawAnnLegend();
                            }
                            scope.$apply();
                        })
                        .on("click", () => {
                            if (!scope.showAnnotations) {
                                return;
                            }
                            angular.element('#modalShower').trigger('click');
                        });
                    annotations.exit().remove();
                }
                var queries = paths.selectAll('.line')
                    .data(scope.data, (d) => { return d.Name; });
                switch (scope.generator) {
                    case 'area':
                        queries.enter()
                            .append('path')
                            .attr('stroke', (d: any) => { return color(d.Name); })
                            .attr('class', 'line')
                            .style('fill', (d: any) => { return color(d.Name); });
                        break;
                    default:
                        queries.enter()
                            .append('path')
                            .attr('stroke', (d: any) => { return color(d.Name); })
                            .attr('class', 'line');
                }
                queries.exit()
                    .remove();

                queries
                    .attr('d', (d: any) => { return line(d.Data); })
                    .attr('transform', null)
                    .transition()
                    .ease('linear')
                    .attr('transform', 'translate(' + (xScale(oldx) - xScale(xdomain[1])) + ')');
                chart.select('.x.brush')
                    .call(brush)
                    .selectAll('rect')
                    .attr('height', height)
                    .on('mouseover', () => {
                        hover.style('display', 'block');
                    })
                    .on('mouseout', () => {
                        hover.style('display', 'none');
                    })
                    .on('mousemove', mousemove);
                chart.select('.x.brush .extent')
                    .style('stroke', '#fff')
                    .style('fill-opacity', '.125')
                    .style('shape-rendering', 'crispEdges');
                oldx = xdomain[1];
                drawLegend(valueIdx);
            };
            var extentStart: string;
            var extentEnd: string;
            var extentDiff: string;
            function brushed() {
                var e: any;
                e = d3.event.sourceEvent;
                if (e.shiftKey) {
                    return;
                }
                var extent = brush.extent();
                extentStart = datefmt(extent[0]);
                extentEnd = datefmt(extent[1]);
                extentDiff = fmtDuration(moment(extent[1]).diff(moment(extent[0])));
                drawLegend(valueIdx);
                if (scope.enableBrush && extentEnd != extentStart) {
                    scope.brushStart = extentStart;
                    scope.brushEnd = extentEnd;
                    scope.$apply();
                }
            }

            function annotateBrushed() {
                if (!scope.annotateEnabled) {
                    return;
                }
                var e: any
                e = d3.event.sourceEvent;
                if (!e.shiftKey) {
                    return;
                }
                var extent = brush.extent();
                scope.annotation = new Annotation();
                scope.annotation.StartDate = moment(extent[0]).utc().format(timeFormat);
                scope.annotation.EndDate = moment(extent[1]).utc().format(timeFormat);
                scope.$apply(); // This logs a console type error, but also works .. odd.
                angular.element('#modalShower').trigger('click');
            }

            var mfmt = 'YYYY/MM/DD-HH:mm:ss';
            function datefmt(d: any) {
                return moment(d).utc().format(mfmt);
            }
        },
    };
}]);
